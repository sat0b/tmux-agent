package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// createPaneStartupDelay is the time to wait after creating a pane
// before sending keys, allowing the TUI to initialize.
var createPaneStartupDelay = 5 * time.Second

// sendKeysTrailingRe matches trailing C-m, Enter, or \n sequences
// that may have been appended literally. These are stripped because
// sendTmuxKeys always sends its own C-m after pasting.
var sendKeysTrailingRe = regexp.MustCompile(`(?i)(\s*(C-m|Enter|\\n))+\s*$`)

// paneInfo holds metadata about a tmux pane running a target command.
type paneInfo struct {
	ID           string
	Command      string
	PID          string
	Dir          string
	LastOutput   string
	LastChangeAt time.Time
}

// isTargetCommand returns true if cmd is a recognized coding agent process.
// The comm field from ps may contain the full path; we check the basename.
func isTargetCommand(cmd string) bool {
	base := cmd
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		base = cmd[i+1:]
	}
	return base == "claude" || base == "codex"
}

// buildProcessTree parses ps output and returns a map of ppid -> child entries.
type psEntry struct {
	pid  string
	comm string
}

func buildProcessTree(psOutput string) map[string][]psEntry {
	tree := make(map[string][]psEntry)
	for _, line := range strings.Split(strings.TrimSpace(psOutput), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, ppid, comm := fields[0], fields[1], fields[2]
		tree[ppid] = append(tree[ppid], psEntry{pid: pid, comm: comm})
	}
	return tree
}

// findTargetDescendant searches the process tree recursively for a target command
// that is a descendant of the given PID.
func findTargetDescendant(tree map[string][]psEntry, pid string) string {
	for _, child := range tree[pid] {
		if isTargetCommand(child.comm) {
			return child.comm
		}
		if found := findTargetDescendant(tree, child.pid); found != "" {
			return found
		}
	}
	return ""
}

// findTargetChild parses ps output and returns the name of the first descendant
// process that is a target command. Searches the full subtree, not just direct children.
func findTargetChild(psOutput string, panePID string) string {
	tree := buildProcessTree(psOutput)
	if found := findTargetDescendant(tree, panePID); found != "" {
		// Return the basename for display.
		if i := strings.LastIndex(found, "/"); i >= 0 {
			return found[i+1:]
		}
		return found
	}
	return ""
}

// lookupChildProcess checks if the pane's shell has a target command as a descendant.
func lookupChildProcess(panePID string) string {
	cmd := exec.Command("ps", "-o", "pid,ppid,comm", "-e")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return findTargetChild(string(out), panePID)
}

// childLookupFn is the function used to find target child processes.
// It can be replaced in tests.
var childLookupFn = lookupChildProcess

// parsePaneList parses tmux list-panes output (tab-separated: id, command, pid, path)
// and returns only panes running a target command.
// If the pane's direct command is not a target, it checks descendant processes.
func parsePaneList(output string) []paneInfo {
	var panes []paneInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		cmd := fields[1]
		pid := fields[2]
		dir := ""
		if len(fields) >= 4 {
			dir = fields[3]
		}
		if !isTargetCommand(cmd) {
			if child := childLookupFn(pid); child != "" {
				cmd = child
			} else {
				continue
			}
		}
		panes = append(panes, paneInfo{
			ID:           fields[0],
			Command:      cmd,
			PID:          pid,
			Dir:          dir,
			LastChangeAt: time.Now(),
		})
	}
	return panes
}

// detectIdle returns true if the pane has been idle longer than the threshold.
func detectIdle(p *paneInfo, threshold time.Duration) bool {
	return time.Since(p.LastChangeAt) >= threshold
}

// statusShort returns a one-line summary like "tmux-agent: 3 active, 1 idle".
func statusShort(panes []paneInfo, threshold time.Duration) string {
	active, idle := 0, 0
	for i := range panes {
		if detectIdle(&panes[i], threshold) {
			idle++
		} else {
			active++
		}
	}
	return fmt.Sprintf("tmux-agent: %d active, %d idle", active, idle)
}

// listTmuxPanes runs tmux list-panes and returns parsed results.
func listTmuxPanes() ([]paneInfo, error) {
	cmd := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id}\t#{pane_current_command}\t#{pane_pid}\t#{pane_current_path}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	return parsePaneList(string(output)), nil
}

// capturePaneOutput captures the last N lines of a tmux pane.
func capturePaneOutput(paneID string, lines int) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", paneID, "-S", fmt.Sprintf("-%d", lines))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane %s: %w", paneID, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// sendTmuxKeys sends text to a tmux pane using send-keys -l (literal mode).
// Newlines are collapsed to spaces and trailing key sequences are stripped.
// After sending the text, C-m is sent twice to submit the input.
func sendTmuxKeys(paneID string, keys string) error {
	keys = strings.ReplaceAll(keys, "\r\n", " ")
	keys = strings.ReplaceAll(keys, "\n", " ")
	keys = strings.ReplaceAll(keys, "\r", " ")
	keys = sendKeysTrailingRe.ReplaceAllString(keys, "")
	keys = strings.TrimSpace(keys)
	if keys == "" {
		return nil
	}

	cmd := exec.Command("tmux", "send-keys", "-t", paneID, "-l", "--", keys)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys -l to %s: %w (output: %s)", paneID, err, string(output))
	}

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		cmd = exec.Command("tmux", "send-keys", "-t", paneID, "C-m")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("tmux send-keys (enter) to %s: %w (output: %s)", paneID, err, string(output))
		}
	}

	return nil
}

// createPaneOpts holds options for creating a new tmux pane.
type createPaneOpts struct {
	Command   string // command to run (e.g., "claude")
	Dir       string // working directory (empty = inherit)
	Session   string // target session (empty = current)
	Split     string // "h" (horizontal, default) or "v" (vertical)
	NewWindow bool   // create as new window instead of split
}

// createTmuxPane creates a new tmux pane running the specified command.
// Returns the pane ID (e.g., "%99").
func createTmuxPane(command string) (string, error) {
	return createTmuxPaneWithOpts(createPaneOpts{Command: command})
}

// createTmuxPaneInDir creates a new tmux pane in the given directory.
func createTmuxPaneInDir(command, dir string) (string, error) {
	return createTmuxPaneWithOpts(createPaneOpts{Command: command, Dir: dir})
}

// createTmuxPaneWithOpts creates a new tmux pane with the given options.
func createTmuxPaneWithOpts(opts createPaneOpts) (string, error) {
	if opts.Command == "" {
		opts.Command = defaultAgentCommand
	}

	var args []string
	if opts.NewWindow {
		args = []string{"new-window"}
		if opts.Session != "" {
			args = append(args, "-t", opts.Session)
		}
	} else {
		splitFlag := "-h"
		if opts.Split == "v" {
			splitFlag = "-v"
		}
		args = []string{"split-window", splitFlag}
		if opts.Session != "" {
			args = append(args, "-t", opts.Session)
		}
	}
	args = append(args, "-P", "-F", "#{pane_id}")
	if opts.Dir != "" {
		args = append(args, "-c", opts.Dir)
	}
	args = append(args, opts.Command)

	cmd := exec.Command("tmux", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		subcmd := args[0]
		return "", fmt.Errorf("tmux %s: %w (output: %s)", subcmd, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// killTmuxPane kills a tmux pane by pane ID.
func killTmuxPane(paneID string) error {
	cmd := exec.Command("tmux", "kill-pane", "-t", paneID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux kill-pane %s: %w (output: %s)", paneID, err, string(output))
	}
	return nil
}

// renameTmuxPane sets the title of a tmux pane.
func renameTmuxPane(paneID, title string) error {
	cmd := exec.Command("tmux", "select-pane", "-t", paneID, "-T", title)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux select-pane -T %s: %w (output: %s)", paneID, err, string(output))
	}
	return nil
}

// sendRawTmuxKeys sends raw tmux key sequences (not literal text) to a pane.
func sendRawTmuxKeys(paneID string, keys ...string) error {
	args := append([]string{"send-keys", "-t", paneID}, keys...)
	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys %s: %w (output: %s)", paneID, err, string(output))
	}
	return nil
}
