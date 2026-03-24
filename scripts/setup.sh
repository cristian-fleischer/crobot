#!/usr/bin/env bash
# CRoBot Setup Wizard
# Interactive setup for CRoBot — AI-powered code review bot.
# https://github.com/cristian-fleischer/crobot
set -euo pipefail

# ---------------------------------------------------------------------------
# Colors & output helpers
# ---------------------------------------------------------------------------

# Respect NO_COLOR (https://no-color.org/)
if [[ -z "${NO_COLOR:-}" ]] && [[ -t 1 ]]; then
  BOLD='\033[1m'
  DIM='\033[2m'
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  BLUE='\033[0;34m'
  CYAN='\033[0;36m'
  RESET='\033[0m'
else
  BOLD='' DIM='' RED='' GREEN='' YELLOW='' BLUE='' CYAN='' RESET=''
fi

info()    { printf '%b▸%b %s\n' "$BLUE" "$RESET" "$*" >&2; }
success() { printf '%b✓%b %s\n' "$GREEN" "$RESET" "$*" >&2; }
warn()    { printf '%b!%b %s\n' "$YELLOW" "$RESET" "$*" >&2; }
error()   { printf '%b✗%b %s\n' "$RED" "$RESET" "$*" >&2; }
header()  { printf '\n%b── %s ──%b\n\n' "${BOLD}${CYAN}" "$*" "$RESET" >&2; }

# ---------------------------------------------------------------------------
# Input helpers
# ---------------------------------------------------------------------------

# When piped via "curl | sh", stdin is the script itself, so interactive
# input must come from /dev/tty instead.
if [[ -t 0 ]]; then
  INPUT=/dev/stdin
else
  if [[ ! -r /dev/tty ]]; then
    error "Cannot read from /dev/tty. Run this script in an interactive terminal."
    exit 1
  fi
  INPUT=/dev/tty
fi

# prompt LABEL [DEFAULT]
# Reads a line of input, showing the default in brackets.
prompt() {
  local label="$1" default="${2:-}"
  if [[ -n "$default" ]]; then
    printf '%b%s%b %b[%s]%b: ' "$BOLD" "$label" "$RESET" "$DIM" "$default" "$RESET" >&2
  else
    printf '%b%s%b: ' "$BOLD" "$label" "$RESET" >&2
  fi
  local value
  read -r value < "$INPUT"
  echo "${value:-$default}"
}

# read_secret LABEL
# Reads a line without echoing (for tokens/passwords).
read_secret() {
  local label="$1"
  printf '%b%s%b: ' "$BOLD" "$label" "$RESET" >&2
  local value
  read -rs value < "$INPUT"
  echo >&2  # newline after hidden input
  echo "$value"
}

# confirm QUESTION [DEFAULT=y]
# Returns 0 for yes, 1 for no.
confirm() {
  local question="$1" default="${2:-y}"
  local hint
  if [[ "$default" == "y" ]]; then hint="Y/n"; else hint="y/N"; fi
  printf '%b%s%b %b[%s]%b: ' "$BOLD" "$question" "$RESET" "$DIM" "$hint" "$RESET" >&2
  local answer
  read -r answer < "$INPUT"
  answer="${answer:-$default}"
  case "${answer,,}" in
    y|yes) return 0 ;;
    *)     return 1 ;;
  esac
}

# select_one HEADER OPTION...
# Prints a numbered menu, reads a choice, prints the selected value.
select_one() {
  local hdr="$1"; shift
  local options=("$@")
  printf '%b%s%b\n' "$BOLD" "$hdr" "$RESET" >&2
  local i
  for i in "${!options[@]}"; do
    printf '  %b%d)%b %s\n' "$CYAN" "$((i + 1))" "$RESET" "${options[$i]}" >&2
  done
  local choice
  while true; do
    printf '%bChoose [1-%d]:%b ' "$DIM" "${#options[@]}" "$RESET" >&2
    read -r choice < "$INPUT"
    if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#options[@]} )); then
      echo "${options[$((choice - 1))]}"
      return
    fi
    warn "Invalid choice. Enter a number between 1 and ${#options[@]}."
  done
}

# select_multi HEADER OPTION...
# Prints a numbered menu where user enters comma-separated choices.
# Returns newline-separated selected values via stdout.
select_multi() {
  local hdr="$1"; shift
  local options=("$@")
  printf '%b%s%b\n' "$BOLD" "$hdr" "$RESET" >&2
  local i
  for i in "${!options[@]}"; do
    printf '  %b%d)%b %s\n' "$CYAN" "$((i + 1))" "$RESET" "${options[$i]}" >&2
  done
  local input
  while true; do
    printf '%bChoose one or more [1-%d, comma-separated]:%b ' "$DIM" "${#options[@]}" "$RESET" >&2
    read -r input < "$INPUT"
    local valid=true
    local selections=()
    IFS=',' read -ra parts <<< "$input"
    for part in "${parts[@]}"; do
      part="$(echo "$part" | tr -d ' ')"
      if [[ "$part" =~ ^[0-9]+$ ]] && (( part >= 1 && part <= ${#options[@]} )); then
        selections+=("${options[$((part - 1))]}")
      else
        valid=false
        break
      fi
    done
    if $valid && (( ${#selections[@]} > 0 )); then
      printf '%s\n' "${selections[@]}"
      return
    fi
    warn "Invalid choice. Enter numbers between 1 and ${#options[@]}, separated by commas."
  done
}

# ---------------------------------------------------------------------------
# Config parsing (simple grep-based, no yq dependency)
# ---------------------------------------------------------------------------

# yaml_get FILE KEY
# Extracts a top-level or one-level-nested YAML value. Very naive but
# sufficient for the flat structure of crobot config files.
yaml_get() {
  local file="$1" key="$2"
  if [[ ! -f "$file" ]]; then
    return
  fi
  # Match "key: value" — strip surrounding quotes and inline comments.
  grep -E "^\s*${key}:" "$file" 2>/dev/null \
    | head -1 \
    | sed -E 's/^[^:]+:\s*//' \
    | sed -E 's/\s*#.*//' \
    | sed -E 's/^["'"'"'](.*?)["'"'"']$/\1/' \
    || true
}

# ---------------------------------------------------------------------------
# State variables (populated by wizard steps)
# ---------------------------------------------------------------------------

scope=""              # global | local | both
platform=""           # github | bitbucket | "" (unset)
setup_platform=""     # which platform we're configuring this run

# Bitbucket
bb_workspace=""
bb_repo=""
bb_user=""
bb_token=""

# GitHub
gh_owner=""
gh_repo=""
gh_token=""

# Usage modes (true/false)
mode_orchestrated="false"
mode_mcp="false"
mode_toolkit="false"

# Agent (new agent being configured this run)
agent_name="claude"
agent_command="claude"
agent_args=""
agent_model=""

# Existing agent block preserved from current config (raw YAML lines).
# New agent is merged into this when writing.
existing_agent_block=""
existing_agent_default=""

# Review
customize_review="false"
max_comments="25"
severity_threshold="warning"

# Credential storage
store_in_file="false"

# Paths
GLOBAL_DIR="$HOME/.config/crobot"
GLOBAL_CONFIG="$GLOBAL_DIR/config.yaml"
LOCAL_CONFIG=".crobot.yaml"

# ---------------------------------------------------------------------------
# Step 0: Install crobot binary (if missing)
# ---------------------------------------------------------------------------

GITHUB_REPO="cristian-fleischer/crobot"

# detect_os prints the OS name matching GoReleaser naming.
detect_os() {
  local os
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    linux*)  echo "linux" ;;
    darwin*) echo "darwin" ;;
    mingw*|msys*|cygwin*) echo "windows" ;;
    *) echo "$os" ;;
  esac
}

# detect_arch prints the architecture matching GoReleaser naming.
detect_arch() {
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "$arch" ;;
  esac
}

# install_dir picks the best writable bin directory.
pick_install_dir() {
  # Prefer /usr/local/bin if writable, else ~/.local/bin.
  if [[ -w "/usr/local/bin" ]]; then
    echo "/usr/local/bin"
  else
    local user_bin="$HOME/.local/bin"
    mkdir -p "$user_bin"
    echo "$user_bin"
  fi
}

step_install() {
  if command -v crobot &>/dev/null; then
    local current_version
    current_version=$(crobot --version 2>/dev/null | grep -oP '\d+\.\d+\.\S+' || echo "unknown")
    info "CRoBot is already installed (v${current_version})."
    return
  fi

  header "Install CRoBot"

  info "CRoBot binary not found on your system."
  if ! confirm "Download and install the latest release?"; then
    warn "Skipping install. You can install manually later."
    warn "See: https://github.com/${GITHUB_REPO}/releases"
    return
  fi

  local os arch
  os=$(detect_os)
  arch=$(detect_arch)

  if [[ "$os" == "windows" ]]; then
    warn "Automatic install is not supported on Windows."
    info "Download manually from: https://github.com/${GITHUB_REPO}/releases"
    return
  fi

  info "Detected platform: ${os}/${arch}"

  # Fetch latest release tag.
  info "Fetching latest release..."
  local release_info tag
  release_info=$(curl -sS "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null) || {
    error "Failed to fetch release info from GitHub."
    return
  }
  tag=$(echo "$release_info" | grep -oP '"tag_name":\s*"\K[^"]+' || true)
  if [[ -z "$tag" ]]; then
    error "Could not determine latest release version."
    return
  fi
  # Strip leading 'v' for the archive name.
  local version="${tag#v}"

  info "Latest release: ${tag}"

  local archive_name="crobot_${version}_${os}_${arch}.tar.gz"
  local download_url="https://github.com/${GITHUB_REPO}/releases/download/${tag}/${archive_name}"

  local install_dir
  install_dir=$(pick_install_dir)

  info "Downloading ${archive_name}..."
  local tmpdir
  tmpdir=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$tmpdir'" EXIT

  if ! curl -sSL -o "${tmpdir}/${archive_name}" "$download_url" 2>/dev/null; then
    error "Download failed. Check your network connection."
    info "URL: $download_url"
    rm -rf "$tmpdir"
    return
  fi

  # Verify we got a real archive (not an HTML error page).
  if ! file "${tmpdir}/${archive_name}" | grep -q 'gzip'; then
    error "Downloaded file is not a valid archive. The release may not exist for ${os}/${arch}."
    info "Download manually from: https://github.com/${GITHUB_REPO}/releases"
    rm -rf "$tmpdir"
    return
  fi

  info "Extracting to ${install_dir}..."
  tar -xzf "${tmpdir}/${archive_name}" -C "$tmpdir"

  if [[ ! -f "${tmpdir}/crobot" ]]; then
    error "Archive did not contain a 'crobot' binary."
    rm -rf "$tmpdir"
    return
  fi

  chmod +x "${tmpdir}/crobot"

  # Use sudo if needed for the install dir.
  if [[ -w "$install_dir" ]]; then
    mv "${tmpdir}/crobot" "${install_dir}/crobot"
  else
    info "Need sudo to install to ${install_dir}."
    sudo mv "${tmpdir}/crobot" "${install_dir}/crobot"
  fi

  rm -rf "$tmpdir"

  # Verify installation.
  if command -v crobot &>/dev/null; then
    success "CRoBot ${tag} installed to ${install_dir}/crobot"
  else
    success "Installed to ${install_dir}/crobot"
    if [[ "$install_dir" == "$HOME/.local/bin" ]]; then
      warn "${install_dir} is not in your PATH."
      info "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
      echo ""
      printf '  %bexport PATH="%s:\$PATH"%b\n' "$DIM" "$install_dir" "$RESET" >&2
      echo ""
    fi
  fi
}

# ---------------------------------------------------------------------------
# Step 1: Detect existing configuration
# ---------------------------------------------------------------------------

step_detect_existing() {
  header "CRoBot Setup"

  local found_global="false" found_local="false"

  if [[ -f "$GLOBAL_CONFIG" ]]; then
    found_global="true"
    info "Found global config: $GLOBAL_CONFIG"
  fi
  if [[ -f "$LOCAL_CONFIG" ]]; then
    found_local="true"
    info "Found local config: $LOCAL_CONFIG"
  fi

  if [[ "$found_global" == "true" ]] || [[ "$found_local" == "true" ]]; then
    echo ""
    local action
    action=$(select_one "What would you like to do?" \
      "Update existing configuration" \
      "Start fresh" \
      "Cancel")

    case "$action" in
      "Cancel")
        info "Setup cancelled."
        exit 0
        ;;
      "Update existing configuration")
        load_existing_defaults "$found_global" "$found_local"
        ;;
      "Start fresh")
        info "Starting with defaults."
        ;;
    esac
  else
    info "No existing configuration found. Let's set things up!"
  fi
}

load_existing_defaults() {
  # Load from global first, then local (local overrides).
  for cfg in "$GLOBAL_CONFIG" "$LOCAL_CONFIG"; do
    if [[ ! -f "$cfg" ]]; then continue; fi

    local val

    val=$(yaml_get "$cfg" "platform")
    if [[ -n "$val" ]]; then platform="$val"; fi

    # Load both platform configs — they can coexist.
    # Load platform-scoped fields using awk to extract per-section values.
    local section_bb section_gh
    section_bb=$(awk '/^bitbucket:/{found=1;next} found&&/^[a-z]/{found=0} found' "$cfg" 2>/dev/null || true)
    section_gh=$(awk '/^github:/{found=1;next} found&&/^[a-z]/{found=0} found' "$cfg" 2>/dev/null || true)

    local v
    v=$(echo "$section_bb" | grep -E '^\s+workspace:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then bb_workspace="$v"; fi
    v=$(echo "$section_bb" | grep -E '^\s+repo:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then bb_repo="$v"; fi
    v=$(echo "$section_bb" | grep -E '^\s+user:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then bb_user="$v"; fi
    v=$(echo "$section_bb" | grep -E '^\s+token:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then bb_token="$v"; fi

    v=$(echo "$section_gh" | grep -E '^\s+owner:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then gh_owner="$v"; fi
    v=$(echo "$section_gh" | grep -E '^\s+repo:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then gh_repo="$v"; fi
    v=$(echo "$section_gh" | grep -E '^\s+token:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
    if [[ -n "$v" ]]; then gh_token="$v"; fi

    # Review settings.
    val=$(yaml_get "$cfg" "max_comments")
    if [[ -n "$val" ]]; then max_comments="$val"; fi
    val=$(yaml_get "$cfg" "severity_threshold")
    if [[ -n "$val" ]]; then severity_threshold="$val"; fi

    # Preserve agent default.
    val=$(yaml_get "$cfg" "default")
    if [[ -n "$val" ]]; then existing_agent_default="$val"; agent_name="$val"; fi

    # Preserve the entire agent.agents block as raw YAML so we can merge
    # the wizard's new agent without losing existing ones.
    local block
    block=$(extract_agent_block "$cfg")
    if [[ -n "$block" ]]; then
      existing_agent_block="$block"

      # Extract command and args for the default agent to pre-fill prompts.
      if [[ -n "$agent_name" ]]; then
        local agent_section
        agent_section=$(echo "$block" | awk -v name="$agent_name" '
          $0 ~ "^    "name":" { found=1; next }
          found && /^    [^ ]/ { found=0 }
          found { print }
        ')
        v=$(echo "$agent_section" | grep -E '^\s+command:' | head -1 | sed -E 's/^[^:]+:\s*//' | sed -E 's/^["'"'"'"]+//' | sed -E 's/["'"'"'"]+$//' || true)
        if [[ -n "$v" ]]; then agent_command="$v"; fi
        # Load args — parse YAML list like: args: ["--flag1", "--flag2"]
        v=$(echo "$agent_section" | grep -E '^\s+args:' | head -1 | sed -E 's/^[^:]+:\s*//' || true)
        if [[ -n "$v" ]]; then
          # Strip brackets and quotes, convert comma-space to space.
          agent_args=$(echo "$v" | tr -d '[]"' | sed 's/,\s*/ /g' | sed 's/^ *//' | sed 's/ *$//')
        fi
      fi
    fi
  done
}

# extract_agent_block FILE
# Extracts the raw "agents:" sub-block (indented entries under agent.agents)
# from a YAML config file. Returns lines like:
#   claude:
#     command: claude-agent-acp
#   gemini:
#     command: gemini
extract_agent_block() {
  local file="$1"
  if [[ ! -f "$file" ]]; then return; fi
  # Capture lines between "  agents:" and the next non-indented/less-indented key.
  awk '
    /^  agents:/ { found=1; next }
    found && /^[^ ]/ { found=0 }
    found && /^  [^ ]/ && !/^    / { found=0 }
    found { print }
  ' "$file"
}

# ---------------------------------------------------------------------------
# Step 2: Scope selection
# ---------------------------------------------------------------------------

step_scope() {
  header "Configuration Scope"

  info "Global config is shared across all repos (~/.config/crobot/config.yaml)."
  info "Local config is per-repo (.crobot.yaml in current directory)."
  echo ""

  scope=$(select_one "Where should configuration be saved?" \
    "Both — global for credentials, local for repo settings (recommended)" \
    "Global only" \
    "Local only")

  case "$scope" in
    "Both"*)  scope="both" ;;
    "Global"*) scope="global" ;;
    "Local"*)  scope="local" ;;
  esac
}

# ---------------------------------------------------------------------------
# Step 3: Platform selection
# ---------------------------------------------------------------------------

step_platform() {
  header "Platform"

  info "Choose which platform to configure credentials for."
  info "You can configure the other platform later by re-running setup."
  if [[ -n "$platform" ]]; then
    info "Current default platform: $platform"
  fi
  echo ""

  setup_platform=$(select_one "Which platform do you want to set up?" \
    "GitHub" \
    "Bitbucket Cloud" \
    "Skip (keep current settings)")

  case "$setup_platform" in
    "GitHub")          setup_platform="github" ;;
    "Bitbucket Cloud") setup_platform="bitbucket" ;;
    "Skip"*)           setup_platform="" ;;
  esac
}

# ---------------------------------------------------------------------------
# Step 4: Platform credentials
# ---------------------------------------------------------------------------

step_credentials() {
  if [[ -z "$setup_platform" ]]; then
    return
  fi

  header "Credentials"

  if [[ "$setup_platform" == "bitbucket" ]]; then
    step_credentials_bitbucket
  else
    step_credentials_github
  fi
}

step_credentials_bitbucket() {
  info "Bitbucket Cloud credentials."
  info "Create an API token at: https://id.atlassian.com/manage-profile/security/api-tokens"
  echo ""
  info "Find your workspace and repo in the url you use to access your bitbucket repo."
  info "URL format: https://bitbucket.org/{workspace}/{repo}/src/master/"
  info "Find your username at: https://bitbucket.org/account/settings/"
  echo ""

  bb_workspace=$(prompt "Workspace slug" "$bb_workspace")

  if [[ "$scope" != "global" ]]; then
    bb_repo=$(prompt "Repository slug" "$bb_repo")
  else
    bb_repo=$(prompt "Repository slug (optional, press Enter to skip)" "$bb_repo")
  fi

  bb_user=$(prompt "User (email or username)" "$bb_user")

  local token_hint=""
  if [[ -n "$bb_token" ]]; then
    token_hint=" (leave blank to keep existing)"
  fi
  local new_token
  new_token=$(read_secret "API token${token_hint}")
  if [[ -n "$new_token" ]]; then
    bb_token="$new_token"
  fi

  if [[ -z "$bb_token" ]]; then
    error "API token is required."
    exit 1
  fi
}

step_credentials_github() {
  info "GitHub credentials."
  info "Create a fine-grained PAT at: https://github.com/settings/personal-access-tokens"""
  info "Required permissions: Pull requests (read/write), Contents (read-only)."
  echo ""
  info "Find your owner and repo in the url you are using to access your GitHub repo."
  info "URL format: https://github.com/{owner}/{repo}"
  echo ""

  gh_owner=$(prompt "Repository owner (user or org)" "$gh_owner")

  if [[ "$scope" != "global" ]]; then
    gh_repo=$(prompt "Repository name" "$gh_repo")
  else
    gh_repo=$(prompt "Repository name (optional, press Enter to skip)" "$gh_repo")
  fi

  local token_hint=""
  if [[ -n "$gh_token" ]]; then
    token_hint=" (leave blank to keep existing)"
  fi
  local new_token
  new_token=$(read_secret "Personal access token${token_hint}")
  if [[ -n "$new_token" ]]; then
    gh_token="$new_token"
  fi

  if [[ -z "$gh_token" ]]; then
    error "Personal access token is required."
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Step 5: Usage modes
# ---------------------------------------------------------------------------

step_modes() {
  header "Usage Modes"

  info "CRoBot supports multiple ways to run reviews:"
  info "  Orchestrated — 'crobot review <pr-url>' drives the full workflow via an AI agent"
  info "  MCP Server   — 'crobot serve --mcp' exposes tools for Claude Code, Cursor, etc."
  info "  CLI Toolkit   — use individual commands (export-pr-context, apply-review-findings, ...)"
  echo ""

  local selections
  selections=$(select_multi "Which modes will you use?" \
    "Orchestrated reviews (crobot review)" \
    "MCP Server (crobot serve --mcp)" \
    "CLI Toolkit (individual commands)")

  mode_orchestrated="false"
  mode_mcp="false"
  mode_toolkit="false"

  while IFS= read -r line; do
    case "$line" in
      "Orchestrated"*) mode_orchestrated="true" ;;
      "MCP Server"*)   mode_mcp="true" ;;
      "CLI Toolkit"*)  mode_toolkit="true" ;;
    esac
  done <<< "$selections"
}

# ---------------------------------------------------------------------------
# Step 6: Agent configuration (orchestrated mode only)
# ---------------------------------------------------------------------------

step_agent() {
  if [[ "$mode_orchestrated" != "true" ]]; then
    return
  fi

  header "Agent Configuration"

  info "CRoBot needs an ACP-compatible agent to run orchestrated reviews."
  echo ""

  agent_name=$(prompt "Agent name" "$agent_name")

  # Show the full command+args as a single prompt for convenience.
  local current_full="$agent_command"
  if [[ -n "$agent_args" ]]; then
    current_full="$agent_command $agent_args"
  fi
  local full_command
  full_command=$(prompt "Agent command (e.g. 'claude' or 'gemini --experimental-acp')" "$current_full")

  # Split into command (first word) and args (rest).
  # shellcheck disable=SC2086
  set -- $full_command
  agent_command="$1"
  shift
  agent_args="$*"

  agent_model=$(prompt "Model ID (optional, leave blank for agent default)" "$agent_model")
}

# ---------------------------------------------------------------------------
# Step 7: Review settings
# ---------------------------------------------------------------------------

step_review() {
  header "Review Settings"

  if ! confirm "Customize review settings? (defaults are usually fine)"; then
    return
  fi

  echo ""
  max_comments=$(prompt "Max comments per review" "$max_comments")

  severity_threshold=$(select_one "Minimum severity threshold" \
    "warning (recommended)" \
    "info (all findings)" \
    "error (critical only)")

  case "$severity_threshold" in
    "warning"*) severity_threshold="warning" ;;
    "info"*)    severity_threshold="info" ;;
    "error"*)   severity_threshold="error" ;;
  esac
}

# ---------------------------------------------------------------------------
# Step 8: Credential storage
# ---------------------------------------------------------------------------

step_credential_storage() {
  header "Credential Storage"

  warn "Tokens are sensitive. Environment variables are the safest option."
  echo ""

  local choice
  choice=$(select_one "How should credentials be stored?" \
    "Environment variables (recommended)" \
    "In the config file")

  case "$choice" in
    "Environment"*) store_in_file="false" ;;
    "In the"*)      store_in_file="true" ;;
  esac
}

# ---------------------------------------------------------------------------
# Step 9: Generate output
# ---------------------------------------------------------------------------

step_generate() {
  header "Generating Configuration"

  case "$scope" in
    global)
      write_global_config
      ;;
    local)
      write_local_config
      ;;
    both)
      write_global_config
      write_local_config
      ;;
  esac

  # Generate .mcp.json if MCP mode selected.
  if [[ "$mode_mcp" == "true" ]]; then
    write_mcp_json
  fi

  # Offer skill installation if toolkit mode selected.
  if [[ "$mode_toolkit" == "true" ]] || [[ "$mode_orchestrated" == "true" ]]; then
    offer_skill_install
  fi

  # Offer philosophy export.
  offer_philosophy_export

  # Print credential env var instructions.
  if [[ "$store_in_file" == "false" ]]; then
    print_env_instructions
  fi

  print_next_steps
}

# should_write_token PLATFORM TOKEN
# Returns 0 (true) if the token should be written to the config file.
# Logic: if this platform was configured this run, respect store_in_file.
# If it wasn't touched, preserve what was there (token exists → write it).
should_write_token() {
  local plat="$1" token="$2"
  if [[ -z "$token" ]]; then return 1; fi
  if [[ "$setup_platform" == "$plat" ]]; then
    # This platform was configured this run — respect the user's choice.
    [[ "$store_in_file" == "true" ]]
  else
    # This platform was NOT configured this run — preserve existing token.
    return 0
  fi
}

write_global_config() {
  mkdir -p "$GLOBAL_DIR"

  local has_secrets="false"

  {
    echo "# CRoBot global configuration"
    echo "# Generated by setup wizard on $(date +%Y-%m-%d)"

    # Write platform default only if set.
    if [[ -n "$platform" ]]; then
      echo ""
      echo "platform: $platform"
    fi

    # Write bitbucket section if it has any data.
    if [[ -n "$bb_workspace" || -n "$bb_user" ]]; then
      echo ""
      echo "bitbucket:"
      if [[ -n "$bb_workspace" ]]; then echo "  workspace: \"$bb_workspace\""; fi
      if [[ -n "$bb_repo" ]]; then echo "  repo: \"$bb_repo\""; fi
      if [[ -n "$bb_user" ]]; then echo "  user: \"$bb_user\""; fi
      if should_write_token "bitbucket" "$bb_token"; then
        echo "  token: \"$bb_token\""
        has_secrets="true"
      elif [[ -n "$bb_token" ]]; then
        # Token exists but user chose env vars — omit from file.
        echo "  # token: set CROBOT_BITBUCKET_TOKEN environment variable"
      fi
    fi

    # Write github section if it has any data.
    if [[ -n "$gh_owner" ]]; then
      echo ""
      echo "github:"
      echo "  owner: \"$gh_owner\""
      if [[ -n "$gh_repo" ]]; then echo "  repo: \"$gh_repo\""; fi
      if should_write_token "github" "$gh_token"; then
        echo "  token: \"$gh_token\""
        has_secrets="true"
      elif [[ -n "$gh_token" ]]; then
        echo "  # token: set CROBOT_GITHUB_TOKEN environment variable"
      fi
    fi

    echo ""
    echo "review:"
    echo "  max_comments: $max_comments"
    echo "  severity_threshold: $severity_threshold"
    echo "  bot_label: crobot"

    # Write agent section: merge existing agents with the new one.
    write_agent_section

  } > "$GLOBAL_CONFIG"

  if [[ "$has_secrets" == "true" ]]; then
    chmod 600 "$GLOBAL_CONFIG"
    success "Wrote global config: $GLOBAL_CONFIG (permissions: 600)"
  else
    success "Wrote global config: $GLOBAL_CONFIG"
  fi
}

# write_agent_section
# Outputs the agent: YAML block, merging existing agents with the wizard's
# new agent. Called from within a { ... } > file redirect.
write_agent_section() {
  local has_new_agent="false"
  if [[ "$mode_orchestrated" == "true" ]] && [[ -n "$agent_name" ]]; then
    has_new_agent="true"
  fi

  # Nothing to write if no existing agents and no new agent.
  if [[ -z "$existing_agent_block" ]] && [[ "$has_new_agent" == "false" ]]; then
    return
  fi

  echo ""
  echo "agent:"

  # Agent default: use the new agent name if configured, else preserve existing.
  local default_agent="${existing_agent_default}"
  if [[ "$has_new_agent" == "true" ]]; then
    default_agent="$agent_name"
  fi
  if [[ -n "$default_agent" ]]; then
    echo "  default: $default_agent"
  fi

  if [[ -n "$agent_model" ]]; then
    echo "  model: $agent_model"
  fi

  echo "  agents:"

  # Write existing agents, skipping the one we're about to add/update.
  if [[ -n "$existing_agent_block" ]]; then
    local skip_agent=""
    if [[ "$has_new_agent" == "true" ]]; then
      skip_agent="$agent_name"
    fi
    local skipping="false"
    while IFS= read -r line; do
      # Detect agent name lines (4-space indented, ending with colon).
      if [[ "$line" =~ ^\ {4}[a-zA-Z] ]]; then
        local name
        name=$(echo "$line" | sed 's/^ *//' | sed 's/:.*//')
        if [[ "$name" == "$skip_agent" ]]; then
          skipping="true"
          continue
        else
          skipping="false"
        fi
      elif [[ "$skipping" == "true" ]]; then
        continue
      fi
      echo "$line"
    done <<< "$existing_agent_block"
  fi

  # Write the new/updated agent.
  if [[ "$has_new_agent" == "true" ]]; then
    echo "    $agent_name:"
    echo "      command: $agent_command"
    if [[ -n "$agent_args" ]]; then
      # Format args as a YAML list: args: ["--flag1", "--flag2"]
      local args_yaml=""
      # shellcheck disable=SC2086
      for arg in $agent_args; do
        if [[ -n "$args_yaml" ]]; then
          args_yaml="$args_yaml, \"$arg\""
        else
          args_yaml="\"$arg\""
        fi
      done
      echo "      args: [$args_yaml]"
    fi
  fi
}

write_local_config() {
  local has_secrets="false"

  {
    echo "# CRoBot local configuration (repo-specific overrides)"
    echo "# Generated by setup wizard on $(date +%Y-%m-%d)"

    if [[ "$scope" == "local" ]]; then
      # Local-only: include everything (same logic as global).
      if [[ -n "$platform" ]]; then
        echo ""
        echo "platform: $platform"
      fi

      if [[ -n "$bb_workspace" || -n "$bb_user" ]]; then
        echo ""
        echo "bitbucket:"
        if [[ -n "$bb_workspace" ]]; then echo "  workspace: \"$bb_workspace\""; fi
        if [[ -n "$bb_repo" ]]; then echo "  repo: \"$bb_repo\""; fi
        if [[ -n "$bb_user" ]]; then echo "  user: \"$bb_user\""; fi
        if should_write_token "bitbucket" "$bb_token"; then
          echo "  token: \"$bb_token\""
          has_secrets="true"
        elif [[ -n "$bb_token" ]]; then
          echo "  # token: set CROBOT_BITBUCKET_TOKEN environment variable"
        fi
      fi

      if [[ -n "$gh_owner" ]]; then
        echo ""
        echo "github:"
        echo "  owner: \"$gh_owner\""
        if [[ -n "$gh_repo" ]]; then echo "  repo: \"$gh_repo\""; fi
        if should_write_token "github" "$gh_token"; then
          echo "  token: \"$gh_token\""
          has_secrets="true"
        elif [[ -n "$gh_token" ]]; then
          echo "  # token: set CROBOT_GITHUB_TOKEN environment variable"
        fi
      fi

      echo ""
      echo "review:"
      echo "  max_comments: $max_comments"
      echo "  severity_threshold: $severity_threshold"
      echo "  bot_label: crobot"

      write_agent_section

    else
      # "Both" scope: local config only has repo-specific overrides.
      # Write repo slugs for whichever platforms have them.
      if [[ -n "$bb_repo" ]]; then
        echo ""
        echo "bitbucket:"
        echo "  repo: \"$bb_repo\""
      fi
      if [[ -n "$gh_repo" ]]; then
        echo ""
        echo "github:"
        echo "  repo: \"$gh_repo\""
      fi
    fi
  } > "$LOCAL_CONFIG"

  if [[ "$has_secrets" == "true" ]]; then
    chmod 600 "$LOCAL_CONFIG"
    check_gitignore
    success "Wrote local config: $LOCAL_CONFIG (permissions: 600)"
  else
    success "Wrote local config: $LOCAL_CONFIG"
  fi

  # Always offer to gitignore .crobot/ (ephemeral diff data), even without secrets.
  if [[ -f ".gitignore" ]] && ! grep -qx '.crobot/' .gitignore 2>/dev/null; then
    if confirm "Add .crobot/ to .gitignore? (recommended -- contains ephemeral review data)"; then
      echo ".crobot/" >> .gitignore
      success "Added .crobot/ to .gitignore"
    else
      warn "Remember to add .crobot/ to .gitignore to avoid committing ephemeral review data."
    fi
  fi
}

check_gitignore() {
  if [[ ! -f ".gitignore" ]]; then
    warn ".gitignore not found. Consider adding .crobot.yaml and .crobot/ to prevent committing secrets and ephemeral data."
    return
  fi
  if ! grep -qx '.crobot.yaml' .gitignore 2>/dev/null; then
    if confirm "Add .crobot.yaml to .gitignore? (recommended since it contains a token)"; then
      echo ".crobot.yaml" >> .gitignore
      success "Added .crobot.yaml to .gitignore"
    else
      warn "Remember to add .crobot.yaml to .gitignore to avoid committing secrets."
    fi
  fi
  if ! grep -qx '.crobot/' .gitignore 2>/dev/null; then
    if confirm "Add .crobot/ to .gitignore? (recommended -- contains ephemeral review data)"; then
      echo ".crobot/" >> .gitignore
      success "Added .crobot/ to .gitignore"
    else
      warn "Remember to add .crobot/ to .gitignore to avoid committing ephemeral review data."
    fi
  fi
}

write_mcp_json() {
  local mcp_file=".mcp.json"

  # Use the platform configured this run, or fall back to the default.
  local mcp_platform="${setup_platform:-$platform}"
  if [[ -z "$mcp_platform" ]]; then
    # Pick whichever platform has data.
    if [[ -n "$gh_owner" ]]; then mcp_platform="github";
    elif [[ -n "$bb_workspace" ]]; then mcp_platform="bitbucket";
    else
      warn "No platform credentials configured. Skipping .mcp.json generation."
      return
    fi
  fi

  local env_entries=""
  if [[ "$mcp_platform" == "bitbucket" ]]; then
    env_entries="\"CROBOT_PLATFORM\": \"bitbucket\""
    env_entries="$env_entries,
        \"CROBOT_BITBUCKET_WORKSPACE\": \"$bb_workspace\""
    if [[ -n "$bb_repo" ]]; then
      env_entries="$env_entries,
        \"CROBOT_BITBUCKET_REPO\": \"$bb_repo\""
    fi
    env_entries="$env_entries,
        \"CROBOT_BITBUCKET_USER\": \"$bb_user\""
  else
    env_entries="\"CROBOT_PLATFORM\": \"github\""
    env_entries="$env_entries,
        \"CROBOT_GITHUB_OWNER\": \"$gh_owner\""
    if [[ -n "$gh_repo" ]]; then
      env_entries="$env_entries,
        \"CROBOT_GITHUB_REPO\": \"$gh_repo\""
    fi
  fi

  cat > "$mcp_file" <<EOF
{
  "mcpServers": {
    "crobot": {
      "command": "crobot",
      "args": ["serve", "--mcp"],
      "env": {
        $env_entries
      }
    }
  }
}
EOF

  success "Wrote MCP config: $mcp_file"
  info "Most agents (Claude Code, Cursor, etc.) pick up .mcp.json automatically."
  info "If yours doesn't, manually add this to your agent's MCP configuration:"
  echo ""
  cat "$mcp_file" >&2
  echo ""
}

offer_skill_install() {
  if ! command -v crobot &>/dev/null; then
    info "Tip: once crobot is installed, run 'crobot export-skill' to install the review skill."
    return
  fi

  echo ""
  if confirm "Install the CRoBot review skill? (installs to .agents/skills/ — works with all agents)"; then
    crobot export-skill
  fi
}

offer_philosophy_export() {
  if ! command -v crobot &>/dev/null; then
    return
  fi

  echo ""
  if confirm "Export the default review philosophy for customization?" "n"; then
    crobot export-philosophy --local
    success "Exported review philosophy to .crobot-philosophy.md"
    info "Edit this file to customize how CRoBot reviews your code."
  fi
}

print_env_instructions() {
  # Only show env var instructions for the platform configured this run.
  if [[ -z "$setup_platform" ]]; then return; fi

  local token=""
  local var_name=""
  if [[ "$setup_platform" == "bitbucket" ]] && [[ -n "$bb_token" ]]; then
    token="$bb_token"
    var_name="CROBOT_BITBUCKET_TOKEN"
  elif [[ "$setup_platform" == "github" ]] && [[ -n "$gh_token" ]]; then
    token="$gh_token"
    var_name="CROBOT_GITHUB_TOKEN"
  fi
  if [[ -z "$token" ]]; then return; fi

  header "Environment Variables"

  info "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
  echo ""
  printf '  %bexport %s="%s"%b\n' "$DIM" "$var_name" "$token" "$RESET" >&2
  echo ""
  info "Then reload your shell: source ~/.bashrc  (or ~/.zshrc)"
}

print_next_steps() {
  header "Setup Complete!"

  info "Next steps:"
  echo ""

  if [[ "$mode_orchestrated" == "true" ]]; then
    echo "  # Run an orchestrated review"
    if [[ -n "$bb_workspace" ]]; then
      printf '  %bcrobot review https://bitbucket.org/%s/%s/pull-requests/1 --dry-run%b\n' "$CYAN" "$bb_workspace" "${bb_repo:-your-repo}" "$RESET"
    fi
    if [[ -n "$gh_owner" ]]; then
      printf '  %bcrobot review https://github.com/%s/%s/pull/1 --dry-run%b\n' "$CYAN" "$gh_owner" "${gh_repo:-your-repo}" "$RESET"
    fi
    echo ""
  fi

  if [[ "$mode_mcp" == "true" ]]; then
    echo "  # Start the MCP server (used by Claude Code, Cursor, etc.)"
    printf '  %bcrobot serve --mcp%b\n' "$CYAN" "$RESET"
    echo ""
  fi

  if [[ "$mode_toolkit" == "true" ]]; then
    echo "  # Export PR context for manual review"
    printf '  %bcrobot export-pr-context --pr 1%b\n' "$CYAN" "$RESET"
    echo ""
  fi

  echo "  # View all commands"
  printf '  %bcrobot --help%b\n' "$CYAN" "$RESET"
  echo ""

  info "Use --dry-run (default) to preview before posting comments."
  info "Add --write when ready to post review comments to the PR."
}

# ---------------------------------------------------------------------------
# Step 10: Validate connection (optional)
# ---------------------------------------------------------------------------

step_validate() {
  # Only offer validation if we configured credentials this run.
  if [[ -z "$setup_platform" ]]; then
    return
  fi

  echo ""
  if ! confirm "Test API connection?" "y"; then
    return
  fi

  info "Testing connection..."

  local response status

  if [[ "$setup_platform" == "bitbucket" ]]; then
    response=$(curl -s -w "\n%{http_code}" -u "${bb_user}:${bb_token}" \
      "https://api.bitbucket.org/2.0/user" 2>/dev/null) || true
  else
    response=$(curl -s -w "\n%{http_code}" \
      -H "Authorization: Bearer $gh_token" \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/user" 2>/dev/null) || true
  fi

  status=$(echo "$response" | tail -1)
  local body
  body=$(echo "$response" | sed '$d')

  if [[ "$status" == "200" ]]; then
    local display_name
    if [[ "$setup_platform" == "bitbucket" ]]; then
      display_name=$(echo "$body" | grep -o '"display_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*: *"//' | sed 's/"$//')
    else
      display_name=$(echo "$body" | grep -o '"login"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*: *"//' | sed 's/"$//')
    fi
    success "Connected successfully! Authenticated as: $display_name"
  elif [[ "$status" == "401" ]] || [[ "$status" == "403" ]]; then
    error "Authentication failed (HTTP $status). Check your credentials."
    if [[ "$setup_platform" == "bitbucket" ]]; then
      info "Verify your API token at: https://id.atlassian.com/manage-profile/security/api-tokens"
    else
      info "Verify your PAT at: https://github.com/settings/personal-access-tokens"
      info "Required permissions: Pull requests (read/write), Contents (read-only)"
    fi
  else
    error "Connection failed (HTTP ${status:-timeout}). Check your network and credentials."
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  step_install
  step_detect_existing
  step_scope
  step_platform
  step_credentials
  step_modes
  step_agent
  step_review
  step_credential_storage
  step_generate
  step_validate
}

main "$@"
