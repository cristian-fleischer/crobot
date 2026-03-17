package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	ansiDim     = "\033[2m"
	ansiReset   = "\033[0m"
	ansiClear   = "\033[K"   // clear to end of line
	ansiSave    = "\033[s"   // save cursor position
	ansiRestore = "\033[u"   // restore cursor position
)

// progressWriter displays a persistent status bar at the bottom of the
// terminal while streaming text scrolls in the area above it.
//
// It uses a terminal scroll region (DECSTBM) to reserve the last two rows:
// a delimiter line and the status line. Text output scrolls naturally
// within the region above; the reserved rows are outside the region and
// never move. The scroll region is set up lazily on the first Write so
// that any prompt output printed beforehand goes to the terminal normally.
type progressWriter struct {
	text   io.Writer // where streamed text goes (mdterm or raw stderr)
	out    *os.File  // raw terminal fd for status + ANSI control
	start  time.Time
	prompt int    // prompt size in bytes
	agent  string // agent name for display
	model  string // model ID for display

	mu          sync.Mutex
	closeOnce   sync.Once
	outputBytes int64
	activity    string
	rows        int  // terminal height
	cols        int  // terminal width
	regionSetup bool // scroll region active

	ticker *time.Ticker
	done   chan struct{}
	isTerm bool
}

func newProgressWriter(text io.Writer, out *os.File, promptSize int, agentName string) *progressWriter {
	pw := &progressWriter{
		text:   text,
		out:    out,
		start:  time.Now(),
		prompt: promptSize,
		agent:  agentName,
		done:   make(chan struct{}),
		isTerm: term.IsTerminal(int(out.Fd())),
	}

	if pw.isTerm {
		cols, rows, err := term.GetSize(int(out.Fd()))
		if err == nil && rows >= 5 {
			pw.rows = rows
			pw.cols = cols
		} else {
			pw.isTerm = false
		}
	}

	if pw.isTerm {
		pw.ticker = time.NewTicker(1 * time.Second)
		go pw.tickLoop()
	}

	return pw
}

// setupRegion activates the scroll region and draws the initial status.
// Caller must hold pw.mu.
func (pw *progressWriter) setupRegion() {
	// Scroll existing content out of the visible area so the review
	// output starts on a clean screen.
	for i := 0; i < pw.rows; i++ {
		fmt.Fprint(pw.out, "\n")
	}
	// Reserve last 2 rows (delimiter + status): scrollable area is rows 1..N-2.
	fmt.Fprintf(pw.out, "\033[1;%dr", pw.rows-2)
	// Cursor to top-left of the scroll region.
	fmt.Fprint(pw.out, "\033[1;1H")
	// Draw delimiter + status on the reserved bottom rows.
	pw.drawStatusRows()
	pw.regionSetup = true
}

// drawStatusRows renders the delimiter and status on the reserved bottom rows.
// Uses save/restore cursor so text output is undisturbed.
// Caller must hold pw.mu.
func (pw *progressWriter) drawStatusRows() {
	fmt.Fprint(pw.out, ansiSave)
	// Delimiter line on row N-1.
	fmt.Fprintf(pw.out, "\033[%d;1H", pw.rows-1)
	fmt.Fprint(pw.out, ansiClear)
	pw.writeDelimiter()
	// Status line on row N.
	fmt.Fprintf(pw.out, "\033[%d;1H", pw.rows)
	fmt.Fprint(pw.out, ansiClear)
	pw.writeStatusContent()
	fmt.Fprint(pw.out, ansiRestore)
}

// writeDelimiter draws a thin line spanning the full terminal width.
func (pw *progressWriter) writeDelimiter() {
	fmt.Fprint(pw.out, ansiDim)
	for i := 0; i < pw.cols; i++ {
		fmt.Fprint(pw.out, "\u2500")
	}
	fmt.Fprint(pw.out, ansiReset)
}

// Write passes data to the text writer. All output is serialised with
// the status-update tick under the mutex, so ANSI sequences never
// interleave on stderr.
func (pw *progressWriter) Write(p []byte) (int, error) {
	if !pw.isTerm {
		return pw.text.Write(p)
	}

	pw.mu.Lock()
	defer pw.mu.Unlock()

	if !pw.regionSetup {
		pw.setupRegion()
	}

	pw.outputBytes += int64(len(p))
	n, err := pw.text.Write(p)

	// Refresh status to keep byte counter current.
	pw.drawStatusRows()

	return n, err
}

// SetActivity updates the current activity label shown in the status line.
func (pw *progressWriter) SetActivity(activity string) {
	pw.mu.Lock()
	pw.activity = activity
	pw.mu.Unlock()
}

// SetModel updates the model ID shown in the status line.
func (pw *progressWriter) SetModel(model string) {
	pw.mu.Lock()
	pw.model = model
	pw.mu.Unlock()
}

func (pw *progressWriter) tickLoop() {
	for {
		select {
		case <-pw.ticker.C:
			pw.mu.Lock()
			if pw.regionSetup {
				pw.drawStatusRows()
			}
			pw.mu.Unlock()
		case <-pw.done:
			return
		}
	}
}

func (pw *progressWriter) writeStatusContent() {
	elapsed := time.Since(pw.start).Truncate(time.Second)
	activity := pw.activity
	if activity == "" {
		activity = "waiting..."
	}

	agentLabel := pw.agent
	if agentLabel == "" {
		agentLabel = "agent"
	}
	if pw.model != "" {
		agentLabel += " (" + pw.model + ")"
	}

	fmt.Fprintf(pw.out, "%s\u23f3 %s | \U0001F916 %s | \U0001F4E4 %s | \U0001F4E5 %s | \u2699\ufe0f %s%s",
		ansiDim, elapsed, agentLabel, formatBytes(pw.prompt), formatBytes(int(pw.outputBytes)), activity, ansiReset)
}

// Finish stops the ticker, tears down the scroll region, and prints a summary.
// It is safe to call Finish multiple times; only the first call takes effect.
func (pw *progressWriter) Finish() {
	pw.closeOnce.Do(func() {
		if pw.ticker != nil {
			pw.ticker.Stop()
			close(pw.done)
		}
	})

	pw.mu.Lock()
	defer pw.mu.Unlock()

	if pw.isTerm && pw.regionSetup {
		// Clear the delimiter and status rows.
		fmt.Fprint(pw.out, ansiSave)
		fmt.Fprintf(pw.out, "\033[%d;1H", pw.rows-1)
		fmt.Fprint(pw.out, ansiClear)
		fmt.Fprintf(pw.out, "\033[%d;1H", pw.rows)
		fmt.Fprint(pw.out, ansiClear)
		fmt.Fprint(pw.out, ansiRestore)

		// Reset scroll region to full terminal.
		fmt.Fprint(pw.out, "\033[r")

		// Move cursor to the bottom and print summary below the text.
		fmt.Fprintf(pw.out, "\033[%d;1H", pw.rows-2)
		elapsed := time.Since(pw.start).Truncate(time.Second)
		fmt.Fprintf(pw.out, "\n%s\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500%s\n", ansiDim, ansiReset)
		fmt.Fprintf(pw.out, "%s\u2713 Review complete in %s (prompt: %s, response: %s)%s\n\n",
			ansiDim, elapsed, formatBytes(pw.prompt), formatBytes(int(pw.outputBytes)), ansiReset)
		pw.regionSetup = false
	}
}

func formatBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
