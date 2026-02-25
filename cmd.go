package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultIdleThreshold = 10 * time.Minute

// parseIntFlag finds a named flag in args and returns its integer value.
// Returns defaultVal if the flag is not present.
func parseIntFlag(args []string, flag string, defaultVal int) (int, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, fmt.Errorf("invalid %s value: %s", flag, args[i+1])
			}
			return n, nil
		}
	}
	return defaultVal, nil
}

// truncateLastLine extracts the last line from output and truncates it to maxLen.
func truncateLastLine(output string, maxLen int) string {
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	last := lines[len(lines)-1]
	if len(last) > maxLen {
		return last[:maxLen-3] + "..."
	}
	return last
}

// runSubcommand dispatches tmux-agent subcommands.
func runSubcommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", usage())
	}

	switch args[0] {
	case "panes":
		return runPanes(os.Stdout)
	case "capture":
		return runCapture(args[1:], os.Stdout)
	case "send":
		return runSend(args[1:], os.Stdout)
	case "create":
		return runCreate(args[1:], os.Stdout)
	case "kill":
		return runKill(args[1:], os.Stdout)
	case "kill-all":
		return runKillAll(os.Stdout)
	case "status":
		return runStatus(args[1:], os.Stdout)
	case "rename":
		return runRename(args[1:], os.Stdout)
	case "logs":
		return runLogs(args[1:], os.Stdout)
	case "broadcast":
		return runBroadcast(args[1:], os.Stdout)
	case "restart":
		return runRestart(args[1:], os.Stdout)
	case "workspace":
		return runWorkspace(args[1:], os.Stdout)
	case "history":
		return runHistory(args[1:], os.Stdout)
	case "diff":
		return runDiff(args[1:], os.Stdout)
	case "watch":
		return runWatch(args[1:])
	default:
		return fmt.Errorf("unknown command: %s\n%s", args[0], usage())
	}
}

func usage() string {
	return `usage: tmux-agent [--claude|--codex] <command>

Global flags:
  --claude                       Use claude for this invocation
  --codex                        Use codex for this invocation
  --set-default-agent <name>     Set the default agent (persisted)

Pane operations:
  panes                          List coding agent panes
  capture <pane_id> [--lines N]  Capture pane output
  history <pane_id> [--lines N]  Capture extended scrollback (default 1000)
  send <pane_id> <text...>       Send text to a pane
  create [options]                Create a new pane
  kill <pane_id>                 Kill a pane
  kill-all                       Kill all coding agent panes
  restart <pane_id>              Restart session in a pane
  rename <pane_id> <title>       Set pane title

Multi-pane operations:
  broadcast <text...>            Send text to all coding agent panes
  diff <pane1> <pane2> [--lines N]  Compare output of two panes
  logs <pane_id> [--file path] [--lines N]  Save pane output to file
  status [--short] [--idle duration]  Show pane status
  watch [options]                 Monitor panes for idle detection

Workspace:
  workspace --repo <owner/repo> [--issue N] [--branch name]  Create worktree + pane

Create options:
  --command <cmd>     Command to run (default: configured agent)
  --keys <text>       Send text after startup
  --session <name>    Target session (default: current)
  --split <h|v>       Split direction: h=horizontal, v=vertical (default: h)
  --new-window        Create as new window instead of split

Watch options:
  --scan <duration>   Scan interval (default: 10s)
  --idle <duration>   Idle threshold (default: 10m)
  --log <path>        Also write output to a log file`
}

// gitBranch returns the current git branch for a directory, or "" on error.
func gitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// shortDir returns a compact directory representation.
// For paths under a ghq root, it returns the repo-relative path (e.g., "sat0b/pulse").
// Otherwise, it returns the last directory component.
func shortDir(dir string) string {
	if dir == "" {
		return ""
	}
	// Try to detect ghq-style path: .../github.com/owner/repo[/...]
	if i := strings.Index(dir, "/github.com/"); i >= 0 {
		return dir[i+len("/github.com/"):]
	}
	return filepath.Base(dir)
}

// runPanes lists all coding agent panes.
func runPanes(w io.Writer) error {
	panes, err := listTmuxPanes()
	if err != nil {
		return err
	}
	if len(panes) == 0 {
		fmt.Fprintln(w, "No coding agent panes found")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PANE\tCOMMAND\tDIR\tBRANCH")
	for i := range panes {
		dir := shortDir(panes[i].Dir)
		branch := gitBranch(panes[i].Dir)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", panes[i].ID, panes[i].Command, dir, branch)
	}
	tw.Flush()
	return nil
}

// runCapture captures pane output.
func runCapture(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent capture <pane_id> [--lines N]")
	}
	paneID := args[0]
	lines, err := parseIntFlag(args[1:], "--lines", 10)
	if err != nil {
		return err
	}

	output, err := capturePaneOutput(paneID, lines)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, output)
	return nil
}

// runSend sends text to a pane.
func runSend(args []string, w io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tmux-agent send <pane_id> <text...>")
	}
	paneID := args[0]
	text := strings.Join(args[1:], " ")
	if err := sendTmuxKeys(paneID, text); err != nil {
		return err
	}
	fmt.Fprintf(w, "Sent to pane %s: %s\n", paneID, text)
	return nil
}

// runCreate creates a new pane.
func runCreate(args []string, w io.Writer) error {
	opts := createPaneOpts{Command: activeAgent}
	var keys string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--command":
			if i+1 < len(args) {
				i++
				opts.Command = args[i]
			}
		case "--keys":
			if i+1 < len(args) {
				i++
				keys = args[i]
			}
		case "--session":
			if i+1 < len(args) {
				i++
				opts.Session = args[i]
			}
		case "--split":
			if i+1 < len(args) {
				i++
				opts.Split = args[i]
			}
		case "--new-window":
			opts.NewWindow = true
		}
	}

	paneID, err := createTmuxPaneWithOpts(opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Created pane %s (%s)\n", paneID, opts.Command)

	if keys != "" {
		time.Sleep(createPaneStartupDelay)
		if err := sendTmuxKeys(paneID, keys); err != nil {
			return fmt.Errorf("created pane %s but failed to send keys: %w", paneID, err)
		}
		fmt.Fprintf(w, "Sent to pane %s: %s\n", paneID, keys)
	}
	return nil
}

// runKill kills a pane.
func runKill(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent kill <pane_id>")
	}
	paneID := args[0]
	if err := killTmuxPane(paneID); err != nil {
		return err
	}
	fmt.Fprintf(w, "Killed pane %s\n", paneID)
	return nil
}

// runKillAll kills all coding agent panes.
func runKillAll(w io.Writer) error {
	panes, err := listTmuxPanes()
	if err != nil {
		return err
	}
	if len(panes) == 0 {
		fmt.Fprintln(w, "No coding agent panes found")
		return nil
	}

	for _, p := range panes {
		if err := killTmuxPane(p.ID); err != nil {
			fmt.Fprintf(w, "Error killing pane %s: %v\n", p.ID, err)
			continue
		}
		fmt.Fprintf(w, "Killed pane %s (%s)\n", p.ID, p.Command)
	}
	return nil
}

// runStatus shows pane status.
func runStatus(args []string, w io.Writer) error {
	short := false
	threshold := defaultIdleThreshold

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--short", "-short":
			short = true
		case "--idle":
			if i+1 < len(args) {
				i++
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return fmt.Errorf("invalid --idle value: %s", args[i])
				}
				threshold = d
			}
		}
	}

	panes, err := listTmuxPanes()
	if err != nil {
		return err
	}

	if len(panes) == 0 {
		fmt.Fprintln(w, "No coding agent panes found")
		return nil
	}

	for i := range panes {
		output, err := capturePaneOutput(panes[i].ID, 5)
		if err == nil {
			panes[i].LastOutput = output
		}
	}

	if short {
		fmt.Fprintln(w, statusShort(panes, threshold))
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PANE\tCOMMAND\tSTATUS\tLAST OUTPUT")
	for i := range panes {
		status := "active"
		if detectIdle(&panes[i], threshold) {
			status = "idle"
		}
		lastLine := truncateLastLine(panes[i].LastOutput, 60)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", panes[i].ID, panes[i].Command, status, lastLine)
	}
	tw.Flush()
	return nil
}

// runRename sets a pane title.
func runRename(args []string, w io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tmux-agent rename <pane_id> <title>")
	}
	paneID := args[0]
	title := strings.Join(args[1:], " ")
	if err := renameTmuxPane(paneID, title); err != nil {
		return err
	}
	fmt.Fprintf(w, "Renamed pane %s to %q\n", paneID, title)
	return nil
}

// runLogs saves pane output to a file.
func runLogs(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent logs <pane_id> [--file <path>] [--lines N]")
	}
	paneID := args[0]
	lines, err := parseIntFlag(args[1:], "--lines", 1000)
	if err != nil {
		return err
	}
	file := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--file" && i+1 < len(args) {
			i++
			file = args[i]
		}
	}

	output, err := capturePaneOutput(paneID, lines)
	if err != nil {
		return err
	}

	if file == "" {
		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, ".config", "tmux-agent", "logs")
		os.MkdirAll(logDir, 0755)
		file = filepath.Join(logDir, fmt.Sprintf("%s-%s.log",
			strings.TrimPrefix(paneID, "%"),
			time.Now().Format("20060102-150405")))
	}

	if err := os.WriteFile(file, []byte(output+"\n"), 0644); err != nil {
		return fmt.Errorf("writing log file: %w", err)
	}
	fmt.Fprintf(w, "Saved pane %s output (%d lines) to %s\n", paneID, lines, file)
	return nil
}

// runBroadcast sends text to all coding agent panes.
func runBroadcast(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent broadcast <text...>")
	}
	text := strings.Join(args, " ")

	panes, err := listTmuxPanes()
	if err != nil {
		return err
	}
	if len(panes) == 0 {
		fmt.Fprintln(w, "No coding agent panes found")
		return nil
	}

	for _, p := range panes {
		if err := sendTmuxKeys(p.ID, text); err != nil {
			fmt.Fprintf(w, "Error sending to pane %s: %v\n", p.ID, err)
			continue
		}
		fmt.Fprintf(w, "Sent to pane %s (%s)\n", p.ID, p.Command)
	}
	return nil
}

// restartDelay is the wait time between restart steps.
var restartDelay = 500 * time.Millisecond

// runRestart restarts a coding agent session in a pane.
func runRestart(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent restart <pane_id>")
	}
	paneID := args[0]

	sendRawTmuxKeys(paneID, "C-c")
	time.Sleep(restartDelay)

	sendRawTmuxKeys(paneID, "/exit", "Enter")
	time.Sleep(restartDelay)

	sendRawTmuxKeys(paneID, activeAgent, "Enter")

	fmt.Fprintf(w, "Restarted session in pane %s\n", paneID)
	return nil
}

// runWorkspace creates a git worktree and a pane in it.
func runWorkspace(args []string, w io.Writer) error {
	var issueNum, repo, branch string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--issue":
			if i+1 < len(args) {
				i++
				issueNum = args[i]
			}
		case "--repo":
			if i+1 < len(args) {
				i++
				repo = args[i]
			}
		case "--branch":
			if i+1 < len(args) {
				i++
				branch = args[i]
			}
		}
	}

	if repo == "" {
		return fmt.Errorf("usage: tmux-agent workspace --repo <owner/repo> [--issue N] [--branch name]")
	}

	// Find repo directory using ghq
	ghqCmd := exec.Command("ghq", "root")
	rootOut, err := ghqCmd.Output()
	if err != nil {
		return fmt.Errorf("ghq root: %w", err)
	}
	ghqRoot := strings.TrimSpace(string(rootOut))
	repoDir := filepath.Join(ghqRoot, "github.com", repo)

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("repository not found: %s", repoDir)
	}

	if branch == "" {
		if issueNum != "" {
			branch = fmt.Sprintf("issue-%s", issueNum)
		} else {
			return fmt.Errorf("either --branch or --issue must be specified")
		}
	}

	// Create worktree
	wtDir := filepath.Join(repoDir, ".worktrees", branch)
	wtCmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", branch, wtDir)
	if output, err := wtCmd.CombinedOutput(); err != nil {
		wtCmd = exec.Command("git", "-C", repoDir, "worktree", "add", wtDir, branch)
		if output2, err2 := wtCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("git worktree add: %w\n%s\n%s", err, string(output), string(output2))
		}
	}

	// Create pane in worktree directory
	paneID, err := createTmuxPaneInDir(activeAgent, wtDir)
	if err != nil {
		return fmt.Errorf("creating pane: %w", err)
	}

	title := branch
	if issueNum != "" {
		title = fmt.Sprintf("#%s", issueNum)
	}
	renameTmuxPane(paneID, title)

	fmt.Fprintf(w, "Created workspace:\n")
	fmt.Fprintf(w, "  Worktree: %s\n", wtDir)
	fmt.Fprintf(w, "  Branch:   %s\n", branch)
	fmt.Fprintf(w, "  Pane:     %s\n", paneID)

	if issueNum != "" {
		time.Sleep(createPaneStartupDelay)
		issueText := fmt.Sprintf("gh issue view %s to review the issue and start working on it", issueNum)
		sendTmuxKeys(paneID, issueText)
		fmt.Fprintf(w, "  Issue:    #%s (sent to pane)\n", issueNum)
	}

	return nil
}

// runHistory captures extended scrollback from a pane.
func runHistory(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tmux-agent history <pane_id> [--lines N]")
	}
	paneID := args[0]
	lines, err := parseIntFlag(args[1:], "--lines", 1000)
	if err != nil {
		return err
	}

	output, err := capturePaneOutput(paneID, lines)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, output)
	return nil
}

// runDiff compares the output of two panes.
func runDiff(args []string, w io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tmux-agent diff <pane1> <pane2> [--lines N]")
	}
	pane1, pane2 := args[0], args[1]
	lines, err := parseIntFlag(args[2:], "--lines", 20)
	if err != nil {
		return err
	}

	out1, err := capturePaneOutput(pane1, lines)
	if err != nil {
		return fmt.Errorf("capturing pane %s: %w", pane1, err)
	}
	out2, err := capturePaneOutput(pane2, lines)
	if err != nil {
		return fmt.Errorf("capturing pane %s: %w", pane2, err)
	}

	fmt.Fprintf(w, "=== Pane %s ===\n%s\n\n=== Pane %s ===\n%s\n", pane1, out1, pane2, out2)
	return nil
}
