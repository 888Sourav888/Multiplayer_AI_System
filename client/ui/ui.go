// Package ui provides ANSI-styled terminal rendering primitives for the
// Multiplayer AI client CLI. It uses raw escape codes (no external deps)
// and enables Virtual Terminal Processing on Windows automatically.
package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── ANSI Escape Code Constants ───────────────────────────────────────────────

const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Foreground colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright foreground
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Background colors
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"

	// Cursor / erase
	ClearLineCode  = "\033[2K\r"
	MoveUp         = "\033[1A"
)

// ─── Windows VT100 Init ───────────────────────────────────────────────────────

// InitTerminal enables ANSI/VT100 processing on Windows (no-op on Unix).
// Call this once at program startup.
func InitTerminal() {
	enableWindowsVT()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func c(codes ...string) string { return strings.Join(codes, "") }

func pad(s string, width int) string {
	l := len([]rune(s))
	if l >= width {
		return s
	}
	return s + strings.Repeat(" ", width-l)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// ─── Brand Header ─────────────────────────────────────────────────────────────

// Header prints the full-width branded startup banner.
func Header(version string) {
	width := 64

	logo := []string{
		"  ███╗   ███╗██╗   ██╗██╗  ████████╗██╗██████╗ ██╗      ██╗ ",
		"  ████╗ ████║██║   ██║██║  ╚══██╔══╝██║██╔══██╗██║      ██║ ",
		"  ██╔████╔██║██║   ██║██║     ██║   ██║██████╔╝██║      ██║ ",
		"  ██║╚██╔╝██║██║   ██║██║     ██║   ██║██╔═══╝ ██║      ██║ ",
		"  ██║ ╚═╝ ██║╚██████╔╝███████╗██║   ██║██║     ███████╗ ██║ ",
		"  ╚═╝     ╚═╝ ╚═════╝ ╚══════╝╚═╝   ╚═╝╚═╝     ╚══════╝ ╚═╝ ",
	}

	topLine := "╭" + strings.Repeat("─", width) + "╮"
	botLine := "╰" + strings.Repeat("─", width) + "╯"

	fmt.Println()
	fmt.Println(c(BrightMagenta, Bold) + topLine + Reset)
	for _, row := range logo {
		fmt.Println(c(BrightMagenta, Bold) + "│" + Reset + c(BrightCyan, Bold) + pad(row, width) + Reset + c(BrightMagenta, Bold) + "│" + Reset)
	}

	// Tagline row
	tagline := "  Multiplayer AI Collaboration System"
	fmt.Println(c(BrightMagenta, Bold) + "│" + Reset + c(Dim, White) + pad(tagline, width) + Reset + c(BrightMagenta, Bold) + "│" + Reset)

	// Version row
	verStr := fmt.Sprintf("  %s  ·  Go Client  ·  %s", version, time.Now().Format("2006-01-02"))
	fmt.Println(c(BrightMagenta, Bold) + "│" + Reset + c(BrightBlack) + pad(verStr, width) + Reset + c(BrightMagenta, Bold) + "│" + Reset)

	fmt.Println(c(BrightMagenta, Bold) + botLine + Reset)
	fmt.Println()
}

// ─── Section Banners ─────────────────────────────────────────────────────────

// Banner prints a styled section heading like: ╭─ Section Name ──╮
func Banner(title string) {
	line := fmt.Sprintf("╭─ %s%s%s%s ", Bold, BrightCyan, title, Reset)
	fill := 44 - len(title)
	if fill < 2 {
		fill = 2
	}
	line += c(BrightBlack) + strings.Repeat("─", fill) + "╮" + Reset
	fmt.Println()
	fmt.Println(line)
}

// BannerClose prints the closing line of a banner section.
func BannerClose() {
	fmt.Println(c(BrightBlack) + "╰" + strings.Repeat("─", 48) + "╯" + Reset)
}

// Divider prints a subtle horizontal divider.
func Divider() {
	fmt.Println(c(BrightBlack) + "  " + strings.Repeat("─", 46) + Reset)
}

// ─── Status Badges ────────────────────────────────────────────────────────────

// StatusBadge returns a colored inline status tag.
func StatusBadge(status string) string {
	switch strings.ToUpper(status) {
	case "ACTIVE":
		return c(Bold, Green) + "● ACTIVE" + Reset
	case "ARCHIVED":
		return c(Bold, Yellow) + "◑ ARCHIVED" + Reset
	case "TERMINATED":
		return c(Bold, Red) + "○ TERMINATED" + Reset
	default:
		return c(Dim, White) + status + Reset
	}
}

// ─── Message Printers ─────────────────────────────────────────────────────────

// Success prints a green success line.
func Success(msg string) {
	fmt.Println(c(Bold, BrightGreen) + "  ✓ " + Reset + c(Green) + msg + Reset)
}

// Successf prints a formatted green success line.
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Error prints a red error line.
func Error(msg string) {
	fmt.Println(c(Bold, BrightRed) + "  ✗ " + Reset + c(Red) + msg + Reset)
}

// Errorf prints a formatted red error line.
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Warn prints a yellow warning line.
func Warn(msg string) {
	fmt.Println(c(Bold, BrightYellow) + "  ⚠ " + Reset + c(Yellow) + msg + Reset)
}

// Warnf prints a formatted yellow warning line.
func Warnf(format string, args ...interface{}) {
	Warn(fmt.Sprintf(format, args...))
}

// Info prints a cyan informational line.
func Info(msg string) {
	fmt.Println(c(Bold, BrightCyan) + "  ℹ " + Reset + c(Cyan) + msg + Reset)
}

// Infof prints a formatted cyan informational line.
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Detail prints a dim secondary detail line.
func Detail(msg string) {
	fmt.Println(c(BrightBlack) + "    " + msg + Reset)
}

// Detailf prints a formatted dim secondary detail line.
func Detailf(format string, args ...interface{}) {
	Detail(fmt.Sprintf(format, args...))
}

// ─── Menu Items ───────────────────────────────────────────────────────────────

// MenuItem prints a numbered, icon-decorated menu option.
func MenuItem(n int, icon, label string) {
	fmt.Printf("  %s%s%d%s%s  %s  %s%s%s\n",
		Bold, BrightBlack, n, Reset,
		BrightBlack+"·"+Reset,
		icon,
		White, label, Reset,
	)
}

// SubMenuItem prints an indented numbered sub-option.
func SubMenuItem(n int, label string) {
	fmt.Printf("    %s%s%d.%s  %s%s%s\n",
		Bold, BrightBlue, n, Reset,
		BrightWhite, label, Reset,
	)
}

// ─── Prompts ──────────────────────────────────────────────────────────────────

// Prompt prints a styled "  › label: " and returns the prefix for user input.
func Prompt(label string) {
	fmt.Printf("%s  ›%s %s%s%s: ", BrightCyan, Reset, Bold, label, Reset)
}

// SubPrompt prints a lighter indented prompt.
func SubPrompt(label string) {
	fmt.Printf("%s    ›%s %s%s%s: ", BrightBlack, Reset, White, label, Reset)
}

// ─── Session Cards ────────────────────────────────────────────────────────────

// SessionCard renders a single session as a styled bordered card.
func SessionCard(idx int, name, id, status, version, lastActive, gitRepo, gitBranch, gitCommit string) string {
	var b strings.Builder
	w := 52

	top := fmt.Sprintf("╭─ %s%s%d. %s%s%s%s ", Bold, BrightCyan, idx, BrightWhite, name, Reset, BrightBlack)
	fill := w - len(fmt.Sprintf("─  %d. %s ", idx, name))
	if fill < 2 {
		fill = 2
	}
	top += strings.Repeat("─", fill) + "╮"
	b.WriteString(c(BrightBlack) + top + Reset + "\n")

	row := func(label, val string) {
		b.WriteString(c(BrightBlack) + "│  " + Reset +
			c(BrightBlack) + pad(label, 14) + Reset +
			c(BrightWhite) + truncate(val, 32) + Reset + "\n")
	}

	row("ID:", c(Dim)+truncate(id, 36)+Reset)
	row("Status:", StatusBadge(status))
	row("Version:", c(BrightYellow)+"v"+version+Reset)
	row("Last Active:", c(Dim)+lastActive+Reset)
	if gitRepo != "" {
		row("Git Repo:", c(Cyan)+truncate(gitRepo, 32)+Reset)
		row("Branch:", c(Cyan)+gitBranch+Reset)
		if len(gitCommit) >= 8 {
			row("Commit:", c(Dim)+gitCommit[:8]+Reset)
		}
	}

	bot := "╰" + strings.Repeat("─", w+2) + "╯"
	b.WriteString(c(BrightBlack) + bot + Reset + "\n")
	return b.String()
}

// SessionDetailCard renders a session detail card (no index).
func SessionDetailCard(name, id, status, version, ownerID, storagePath, lastActive, gitRepo, gitBranch, gitCommit string) string {
	var b strings.Builder
	w := 54

	top := fmt.Sprintf("╭─ %s%sSession: %s%s%s%s ", Bold, BrightMagenta, BrightWhite, name, Reset, BrightBlack)
	fill := w - len(fmt.Sprintf("─  Session: %s ", name))
	if fill < 2 {
		fill = 2
	}
	top += strings.Repeat("─", fill) + "╮"
	b.WriteString(c(BrightBlack) + top + Reset + "\n")

	row := func(label, val string) {
		b.WriteString(c(BrightBlack) + "│  " + Reset +
			c(BrightBlack) + pad(label, 16) + Reset +
			val + "\n")
	}

	row("ID:", c(Dim)+id+Reset)
	row("Status:", StatusBadge(status))
	row("Version:", c(BrightYellow)+"v"+version+Reset)
	if ownerID != "" {
		row("Owner:", c(Dim)+ownerID+Reset)
	}
	if storagePath != "" {
		row("Storage:", c(Dim)+truncate(storagePath, 36)+Reset)
	}
	row("Last Active:", c(Dim)+lastActive+Reset)
	if gitRepo != "" {
		b.WriteString(c(BrightBlack) + "│  " + Reset + "\n")
		row("Git Repo:", c(Cyan)+truncate(gitRepo, 36)+Reset)
		row("Branch:", c(Cyan)+gitBranch+Reset)
		if len(gitCommit) >= 8 {
			row("Commit:", c(Dim)+gitCommit[:8]+"…"+Reset)
		}
	}

	bot := "╰" + strings.Repeat("─", w+2) + "╯"
	b.WriteString(c(BrightBlack) + bot + Reset + "\n")
	return b.String()
}

// ─── Spinner ──────────────────────────────────────────────────────────────────

// Spinner is an animated terminal spinner for blocking operations.
type Spinner struct {
	mu      sync.Mutex
	msg     string
	running bool
	done    chan struct{}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates and immediately starts a spinner with the given message.
func NewSpinner(msg string) *Spinner {
	s := &Spinner{
		msg:  msg,
		done: make(chan struct{}),
	}
	s.start()
	return s
}

func (s *Spinner) start() {
	s.running = true
	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				return
			default:
				frame := spinnerFrames[i%len(spinnerFrames)]
				fmt.Fprintf(os.Stderr, "\r%s%s%s %s%s%s  ",
					BrightCyan, frame, Reset,
					Dim, s.msg, Reset)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the line, then optionally prints a result.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.running = false
		close(s.done)
		time.Sleep(90 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", len(s.msg)+8))
	}
}

// StopSuccess halts the spinner and prints a success message.
func (s *Spinner) StopSuccess(msg string) {
	s.Stop()
	Success(msg)
}

// StopError halts the spinner and prints an error message.
func (s *Spinner) StopError(msg string) {
	s.Stop()
	Error(msg)
}

// StopInfo halts the spinner and prints an info message.
func (s *Spinner) StopInfo(msg string) {
	s.Stop()
	Info(msg)
}

// ─── Git Info Display ─────────────────────────────────────────────────────────

// GitInfoBox renders a pretty box with git repository metadata.
func GitInfoBox(repoURL, branch, commitSHA string) {
	fmt.Println(c(BrightBlack) + "  ╭─ " + Reset + c(Bold, Yellow) + "⎇  Git Repository Detected" + Reset + c(BrightBlack) + " ──────────────────╮" + Reset)
	printGitRow := func(label, val string) {
		fmt.Printf(c(BrightBlack)+"  │  "+Reset+c(BrightBlack)+"%-10s"+Reset+" %s\n", label, val)
	}
	printGitRow("URL:", c(Cyan)+truncate(repoURL, 42)+Reset)
	printGitRow("Branch:", c(BrightGreen)+branch+Reset)
	sha := commitSHA
	if len(sha) > 8 {
		sha = sha[:8]
	}
	printGitRow("Commit:", c(Dim)+sha+Reset)
	fmt.Println(c(BrightBlack) + "  ╰" + strings.Repeat("─", 52) + "╯" + Reset)
}

// ─── Misc Utilities ───────────────────────────────────────────────────────────

// Nl prints a blank line.
func Nl() { fmt.Println() }

// ClearLine erases the current terminal line (useful after a spinner).
func ClearLine() {
	fmt.Print(ClearLineCode)
}

// StartupInfo prints the "Starting client" line in a styled way.
func StartupInfo(userID, backendURL string) {
	fmt.Println(c(BrightBlack) + "  ┌─────────────────────────────────────────────────────────┐" + Reset)
	fmt.Printf(c(BrightBlack)+"  │"+Reset+"  "+c(Bold, BrightCyan)+"User"+Reset+"    %s%-36s%s"+c(BrightBlack)+"│"+Reset+"\n",
		BrightWhite, truncate(userID, 36), Reset)
	fmt.Printf(c(BrightBlack)+"  │"+Reset+"  "+c(Bold, BrightCyan)+"Backend"+Reset+" %s%-36s%s"+c(BrightBlack)+"│"+Reset+"\n",
		Dim, truncate(backendURL, 36), Reset)
	fmt.Println(c(BrightBlack) + "  └─────────────────────────────────────────────────────────┘" + Reset)
	fmt.Println()
}

// LeaveMsg prints a styled goodbye message.
func LeaveMsg() {
	fmt.Println()
	fmt.Println(c(BrightMagenta, Bold) + "  ◈  Goodbye! Thanks for using Multiplayer AI." + Reset)
	fmt.Println(c(BrightBlack) + "     Session closed." + Reset)
	fmt.Println()
}

// ConnectingMsg prints a styled connecting message.
func ConnectingMsg(sessionID string) {
	fmt.Println()
	Info(fmt.Sprintf("Connecting to live session  %s%s%s", Dim, truncate(sessionID, 24), Reset))
}

// ActiveSessionHeader prints the in-session loop banner.
func ActiveSessionHeader(sessionName string) {
	fmt.Println()
	w := 48
	name := truncate(sessionName, 30)
	line := fmt.Sprintf("╭─ %s%s● LIVE%s  %s%s%s ", BrightGreen, Bold, Reset+BrightBlack, BrightWhite+Bold, name, Reset+BrightBlack)
	fill := w - len(fmt.Sprintf("─  ● LIVE   %s ", name))
	if fill < 2 {
		fill = 2
	}
	line += strings.Repeat("─", fill) + "╮"
	fmt.Println(c(BrightBlack) + line + Reset)
}
