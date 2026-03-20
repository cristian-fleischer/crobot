package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cristian-fleischer/crobot/internal/agent"
	"github.com/cristian-fleischer/crobot/internal/config"
	"github.com/cristian-fleischer/crobot/internal/platform"
	localplatform "github.com/cristian-fleischer/crobot/internal/platform/local"
	"github.com/cristian-fleischer/crobot/internal/review"
	"github.com/mkozhukh/mdterm"
	"github.com/spf13/cobra"
)

// ReviewOpts holds all resolved options needed to run a review.
type ReviewOpts struct {
	PRRequest       platform.PRRequest
	Platform        platform.Platform
	AgentCfg        *agent.RunConfig
	Config          config.Config
	ModelID         string
	Instructions    string
	Philosophy      string
	MaxComments     int
	DryRun          bool
	ShowAgentOutput bool
	RawOutput       bool
	LocalMode       bool
}

// newReviewCmd creates the review subcommand.
func newReviewCmd() *cobra.Command {
	var (
		workspace       string
		repo            string
		prFlag          string
		agentName       string
		modelFlag       string
		dryRun          bool
		write           bool
		maxComments     int
		timeoutSecs     int
		showAgentOutput bool
		rawOutput       bool
		instructions    string
		agentCommand    string
		philosophyFlag  string
		baseBranch      string
	)

	cmd := &cobra.Command{
		Use:   "review [pr-url-or-number]",
		Short: "Run an AI-powered code review on a pull request or local changes",
		Long: `Spawns an ACP-compatible agent to review a PR and post inline comments.

The PR can be specified as a positional argument or via --pr:
  crobot review https://bitbucket.org/team/repo/pull-requests/42
  crobot review 42
  crobot review --pr 42

When a URL is provided, the workspace, repo, and PR number are extracted
automatically, so --workspace and --repo are not needed.

When no PR is specified, CRoBot enters local mode and reviews all changes
(committed, staged, and unstaged) relative to a base branch. Local mode
always runs as dry-run and renders findings to the terminal.`,
		Example: `  # Review local changes against master (default)
  crobot review

  # Review local changes against a different base branch
  crobot review --base main

  # Using a PR URL (workspace and repo deduced automatically)
  crobot review https://bitbucket.org/myteam/my-service/pull-requests/42

  # Using a PR number (requires workspace and repo from config or flags)
  crobot review --workspace myteam --repo my-service 42

  # With a specific agent
  crobot review https://bitbucket.org/myteam/my-service/pull-requests/42 --agent claude

  # Write comments and see agent output
  crobot review 42 --write --show-agent-output

  # --pr flag also works
  crobot review --pr 42`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun && write {
				return fmt.Errorf("--dry-run and --write are mutually exclusive")
			}

			isDryRun := !write

			// Resolve PR reference from positional arg or --pr flag.
			prValue := prFlag
			if len(args) > 0 {
				if prValue != "" {
					return fmt.Errorf("specify the PR as a positional argument or --pr, not both")
				}
				prValue = args[0]
			}

			// Local mode: no PR specified — review local git changes.
			isLocalMode := prValue == ""

			// 1. Load config.
			cfg, err := config.LoadDefault()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			var pr *platform.PRRequest
			var plat platform.Platform

			if isLocalMode {
				localPlat := localplatform.New(baseBranch, ".")
				plat = localPlat
				isDryRun = true // local mode never posts
				pr = &platform.PRRequest{
					Workspace: "local",
					Repo:      localPlat.RepoName(),
					PRNumber:  0,
				}
			} else {
				// 2. Resolve PR reference (URL or number).
				pr, err = resolvePRFlag(prValue, workspace, repo, cfg)
				if err != nil {
					return err
				}

				// 3. Auto-detect platform from URL when applicable.
				if platform.IsPRURL(prValue) {
					if detected := platform.PlatformFromURL(prValue); detected != "" {
						cfg.Platform = detected
					}
				}

				// 4. Build platform client.
				plat, err = buildPlatform(cfg)
				if err != nil {
					return fmt.Errorf("creating platform client: %w", err)
				}
			}

			// 5. Resolve agent config.
			agentCfg, err := resolveAgentConfig(cfg, agentName, agentCommand)
			if err != nil {
				return fmt.Errorf("resolving agent config: %w", err)
			}

			// 5b. Override timeout if --timeout flag was provided.
			if cmd.Flags().Changed("timeout") && timeoutSecs > 0 {
				agentCfg.Timeout = time.Duration(timeoutSecs) * time.Second
			}

			// 6. Resolve max comments: CLI flag > config default.
			mc := cfg.Review.MaxComments
			if cmd.Flags().Changed("max-comments") {
				mc = maxComments
			}

			// 7. Resolve model: --model flag > CROBOT_MODEL env > config.
			modelID := modelFlag
			if modelID == "" {
				modelID = cfg.Agent.Model
			}

			// 8. Resolve review philosophy: CLI flag > env > config > file convention > default.
			if cmd.Flags().Changed("review-philosophy") {
				cfg.Review.PhilosophyPath = philosophyFlag
			}
			philosophy, err := config.LoadPhilosophy(cfg)
			if err != nil {
				return fmt.Errorf("loading review philosophy: %w", err)
			}

			opts := ReviewOpts{
				PRRequest:       *pr,
				Platform:        plat,
				AgentCfg:        agentCfg,
				Config:          cfg,
				ModelID:         modelID,
				Instructions:    instructions,
				Philosophy:      philosophy,
				MaxComments:     mc,
				DryRun:          isDryRun,
				ShowAgentOutput: showAgentOutput,
				RawOutput:       rawOutput,
				LocalMode:       isLocalMode,
			}

			result, err := runReview(cmd.Context(), opts)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling result: %w", err)
			}
			return writeJSON(os.Stdout, data)
		},
	}

	cmd.Flags().StringVar(&prFlag, "pr", "", "Pull request number or URL (e.g. 42 or https://bitbucket.org/team/repo/pull-requests/42)")
	cmd.Flags().StringVar(&agentName, "agent", "", "ACP agent name (from config)")
	cmd.Flags().StringVar(&agentCommand, "agent-command", "", "ACP agent binary to run directly (bypasses config)")
	cmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Model ID to use (or \"ask\" for interactive selection)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and preview without posting")
	cmd.Flags().BoolVar(&write, "write", false, "Post review comments")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace or organization slug")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository slug")
	cmd.Flags().IntVar(&maxComments, "max-comments", 0, "Maximum number of comments to post (0 = unlimited; omit to use config default)")
	cmd.Flags().IntVarP(&timeoutSecs, "timeout", "t", 0, "Agent timeout in seconds (0 = use config default, currently 10m)")
	cmd.Flags().BoolVar(&showAgentOutput, "show-agent-output", false, "Show the agent's stderr output during the review")
	cmd.Flags().BoolVar(&rawOutput, "raw", false, "Disable markdown formatting of agent output (use with --show-agent-output)")
	cmd.Flags().StringVarP(&instructions, "instructions", "i", "", "Additional instructions appended to the review prompt")
	cmd.Flags().StringVar(&philosophyFlag, "review-philosophy", "", "Path to a custom review philosophy markdown file")
	cmd.Flags().StringVar(&baseBranch, "base", "master", "Base branch for local review (used when no PR is specified)")
	return cmd
}

// runReview executes the full AI-powered review pipeline: fetch PR context,
// run the agent, extract findings, and post comments via the review engine.
func runReview(ctx context.Context, opts ReviewOpts) (*review.ReviewResult, error) {
	slog.Debug("starting review",
		"workspace", opts.PRRequest.Workspace, "repo", opts.PRRequest.Repo,
		"pr", opts.PRRequest.PRNumber, "agent", opts.AgentCfg.Name,
		"dry_run", opts.DryRun,
	)
	slog.Debug("resolved agent", "name", opts.AgentCfg.Name, "command", opts.AgentCfg.Command, "timeout", opts.AgentCfg.Timeout)

	// 1. Fetch PR context.
	prCtx, err := opts.Platform.GetPRContext(ctx, opts.PRRequest)
	if err != nil {
		return nil, fmt.Errorf("fetching PR context: %w", err)
	}

	// 1b. Write per-file diffs to disk for incremental agent consumption.
	if err := platform.CleanupStaleDiffDirs(".crobot"); err != nil {
		slog.Warn("failed to clean stale diff dirs", "error", err)
	}
	stats := platform.ComputeDiffStats(prCtx.DiffHunks)
	diffDir := platform.NewDiffDir(".crobot")
	if err := platform.WriteDiffFiles(prCtx.DiffHunks, stats, diffDir); err != nil {
		return nil, fmt.Errorf("writing diff files: %w", err)
	}
	defer platform.CleanupDiffDir(diffDir)

	// 2. Build review prompt (with custom philosophy if provided).
	prompt := agent.BuildFullPromptWithPhilosophy(prCtx, &opts.PRRequest, opts.Philosophy, diffDir)
	if opts.Instructions != "" {
		prompt += "\n\n## Additional Instructions\n\n" + opts.Instructions + "\n"
	}

	// 3. Create and start agent client.
	clientCfg := agent.ClientConfig{
		Command: opts.AgentCfg.Command,
		Args:    opts.AgentCfg.Args,
		Timeout: opts.AgentCfg.Timeout,
	}
	client := agent.NewClient(clientCfg)

	agentCtx, cancel := context.WithTimeout(ctx, opts.AgentCfg.Timeout)
	defer cancel()

	if err := client.Start(agentCtx); err != nil {
		return nil, fmt.Errorf("starting agent: %w", err)
	}
	defer client.Close()

	// 4. Create session, initialize, and send prompt.
	fsHandler, err := agent.NewFSHandler(prCtx.HeadCommit, ".")
	if err != nil {
		return nil, fmt.Errorf("creating fs handler: %w", err)
	}
	sessionCfg := agent.SessionConfig{
		Client:    client,
		FSHandler: fsHandler,
	}
	// Don't pass "ask" as a model ID -- it's handled after session creation.
	if opts.ModelID != "" && opts.ModelID != "ask" {
		sessionCfg.ModelID = opts.ModelID
	}

	var pw *progressWriter
	if opts.ShowAgentOutput {
		var textWriter io.Writer
		if opts.RawOutput {
			textWriter = os.Stderr
		} else {
			textWriter = mdterm.New(os.Stderr)
		}
		pw = newProgressWriter(textWriter, os.Stderr, len(prompt), opts.AgentCfg.Name)
		sessionCfg.StreamWriter = pw
		sessionCfg.ActivityFunc = pw.SetActivity
	}

	session := agent.NewSession(sessionCfg)

	if err := session.Initialize(agentCtx); err != nil {
		if pw != nil {
			pw.Finish()
		}
		return nil, fmt.Errorf("initializing agent session: %w", err)
	}
	defer session.Close(agentCtx)

	// Create the ACP session early to get model metadata.
	if err := session.CreateSession(agentCtx); err != nil {
		if pw != nil {
			pw.Finish()
		}
		return nil, fmt.Errorf("creating agent session: %w", err)
	}

	// Interactive model selection: --model ask
	if opts.ModelID == "ask" {
		chosen, err := promptModelSelection(session)
		if err != nil {
			if pw != nil {
				pw.Finish()
			}
			return nil, fmt.Errorf("model selection: %w", err)
		}
		// Close current session, set the chosen model, create a new one.
		session.Close(agentCtx)
		session.SetModel(chosen)
		if err := session.CreateSession(agentCtx); err != nil {
			if pw != nil {
				pw.Finish()
			}
			return nil, fmt.Errorf("creating session with model %q: %w", chosen, err)
		}
	}

	// Update progress bar with model info from the session.
	if pw != nil && session.CurrentModel != "" {
		pw.SetModel(session.CurrentModel)
	}

	slog.Debug("sending review prompt to agent", "prompt_length", len(prompt), "model", session.CurrentModel)
	slog.Debug("review prompt content", "prompt", prompt)
	if opts.ShowAgentOutput {
		fmt.Fprintf(os.Stderr, "\n--- PROMPT SENT TO AGENT ---\n")
		if opts.RawOutput {
			fmt.Fprintf(os.Stderr, "%s", prompt)
		} else {
			mdterm.New(os.Stderr).Render(prompt)
		}
		fmt.Fprintf(os.Stderr, "\n--- END PROMPT ---\n\n")
	}
	result, err := session.Prompt(agentCtx, prompt)
	if pw != nil {
		pw.Finish()
	}
	if err != nil {
		return nil, fmt.Errorf("agent prompt: %w", err)
	}

	// 5. Extract findings from agent response.
	findings, err := agent.ExtractFindings(result.FinalText)
	if err != nil {
		return nil, fmt.Errorf("extracting findings: %w", err)
	}

	slog.Debug("extracted findings", "count", len(findings))

	// 6. Run review engine.
	engine := review.NewEngine(opts.Platform, review.EngineConfig{
		MaxComments:       opts.MaxComments,
		DryRun:            opts.DryRun,
		BotLabel:          opts.Config.Review.BotLabel,
		SeverityThreshold: opts.Config.Review.SeverityThreshold,
	})

	engineResult, err := engine.RunWithContext(ctx, opts.PRRequest, prCtx, findings)
	if err != nil {
		return nil, fmt.Errorf("running review engine: %w", err)
	}

	// 7. Print rendered comments to stderr for human preview.
	if (opts.LocalMode || opts.ShowAgentOutput) && len(engineResult.Posted) > 0 {
		RenderFindings(engineResult.Posted, prCtx.DiffHunks, os.Stderr, opts.RawOutput)
	}

	return engineResult, nil
}

// resolvePRFlag parses the --pr flag value as either a URL or a number,
// merging with workspace/repo from flags and config.
func resolvePRFlag(prFlag, workspace, repo string, cfg config.Config) (*platform.PRRequest, error) {
	if platform.IsPRURL(prFlag) {
		ref, err := platform.ParsePRURL(prFlag)
		if err != nil {
			return nil, fmt.Errorf("parsing PR URL: %w", err)
		}
		// URL-derived values can be overridden by explicit flags.
		if workspace != "" {
			ref.Workspace = workspace
		}
		if repo != "" {
			ref.Repo = repo
		}
		return ref, nil
	}

	// Treat as a numeric PR ID.
	prNum, err := strconv.Atoi(prFlag)
	if err != nil || prNum <= 0 {
		return nil, fmt.Errorf("--pr must be a positive number or a pull request URL, got %q", prFlag)
	}

	workspace, repo = resolveWorkspaceRepo(workspace, repo, cfg)
	if workspace == "" || repo == "" {
		return nil, fmt.Errorf("--workspace and --repo are required when --pr is a number (or use a PR URL)")
	}

	return &platform.PRRequest{
		Workspace: workspace,
		Repo:      repo,
		PRNumber:  prNum,
	}, nil
}

// promptModelSelection presents available models and asks the user to choose.
func promptModelSelection(session *agent.Session) (string, error) {
	models := session.AvailableModels
	if len(models) == 0 {
		return "", fmt.Errorf("agent does not report any available models")
	}

	current := session.CurrentModel
	fmt.Fprintf(os.Stderr, "\nAvailable models (current: %s):\n\n", current)
	for i, m := range models {
		marker := "  "
		if m.ID == current {
			marker = "> "
		}
		desc := m.Name
		if m.Description != "" && m.Description != m.Name {
			desc += " - " + m.Description
		}
		fmt.Fprintf(os.Stderr, "  %s%d) %-28s %s\n", marker, i+1, m.ID, desc)
	}
	fmt.Fprintf(os.Stderr, "\nSelect model [1-%d]: ", len(models))

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("no input received")
	}
	input := strings.TrimSpace(scanner.Text())

	// Accept a number or a model ID directly.
	if n, err := strconv.Atoi(input); err == nil {
		if n < 1 || n > len(models) {
			return "", fmt.Errorf("selection %d out of range [1-%d]", n, len(models))
		}
		return models[n-1].ID, nil
	}

	// Check if input matches a model ID.
	for _, m := range models {
		if m.ID == input {
			return m.ID, nil
		}
	}
	return "", fmt.Errorf("unknown model %q", input)
}
