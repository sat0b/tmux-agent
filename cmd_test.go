package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helper function tests ---

func TestParseIntFlag(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		flag       string
		defaultVal int
		want       int
		wantErr    bool
	}{
		{"found", []string{"--lines", "20"}, "--lines", 10, 20, false},
		{"not found", []string{"--other", "5"}, "--lines", 10, 10, false},
		{"empty args", nil, "--lines", 10, 10, false},
		{"invalid value", []string{"--lines", "abc"}, "--lines", 10, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIntFlag(tt.args, tt.flag, tt.defaultVal)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTruncateLastLine(t *testing.T) {
	tests := []struct {
		name   string
		output string
		maxLen int
		want   string
	}{
		{"empty", "", 60, ""},
		{"short", "hello", 60, "hello"},
		{"multiline", "line1\nline2\nline3", 60, "line3"},
		{"truncated", "a very long line that exceeds the maximum length allowed here", 20, "a very long line ..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLastLine(tt.output, tt.maxLen)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- send subcommand tests ---

func TestRunSend(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runSend([]string{"%5", "hello", "world"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "%5") {
		t.Errorf("expected pane ID in output, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "send-keys") {
		t.Errorf("expected send-keys in tmux args, got: %s", args)
	}
	if !strings.Contains(args, "%5") {
		t.Errorf("expected pane ID in tmux args, got: %s", args)
	}
}

func TestRunSend_MissingArgs(t *testing.T) {
	var buf bytes.Buffer

	err := runSend(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing args")
	}

	err = runSend([]string{"%5"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
}

// --- panes subcommand tests ---

func TestRunPanes(t *testing.T) {
	dir := t.TempDir()

	// Create a git repo in tmpdir so gitBranch returns something
	gitScript := filepath.Join(dir, "git")
	os.WriteFile(gitScript, []byte(`#!/bin/sh
echo "main"
`), 0755)

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  list-panes)
    printf "%%3\tclaude\t12345\t/home/user/ghq/github.com/owner/repo\n%%5\tcodex\t12346\t/tmp/work\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runPanes(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "%3") {
		t.Errorf("expected pane %%3 in output, got: %s", output)
	}
	if !strings.Contains(output, "claude") {
		t.Errorf("expected 'claude' in output, got: %s", output)
	}
	if !strings.Contains(output, "owner/repo") {
		t.Errorf("expected 'owner/repo' in output, got: %s", output)
	}
	if !strings.Contains(output, "DIR") {
		t.Errorf("expected DIR header in output, got: %s", output)
	}
}

func TestRunPanes_NoPanes(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  list-panes)
    printf "%%1\tbash\t11111\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runPanes(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No coding agent panes found") {
		t.Errorf("expected no panes message, got: %s", output)
	}
}

// --- capture subcommand tests ---

func TestRunCapture(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  capture-pane)
    echo "line1"
    echo "line2"
    echo "line3"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runCapture([]string{"%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "line1") {
		t.Errorf("expected captured output, got: %s", output)
	}
}

func TestRunCapture_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runCapture(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

func TestRunCapture_CustomLines(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
echo "captured"
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runCapture([]string{"%5", "--lines", "20"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	if !strings.Contains(string(data), "-20") {
		t.Errorf("expected -20 in tmux args, got: %s", string(data))
	}
}

// --- kill subcommand tests ---

func TestRunKill(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" > `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runKill([]string{"%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "%5") {
		t.Errorf("expected pane ID in output, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	if !strings.Contains(string(data), "kill-pane") {
		t.Errorf("expected kill-pane in tmux args, got: %s", string(data))
	}
}

func TestRunKill_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runKill(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}

// --- create subcommand tests ---

func TestRunCreate(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  split-window)
    echo "%99"
    ;;
  send-keys)
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runCreate(nil, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "%99") {
		t.Errorf("expected pane ID in output, got: %s", output)
	}
}

// --- rename subcommand tests ---

func TestRunRename(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" > `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runRename([]string{"%5", "my-task"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Renamed pane %5") {
		t.Errorf("expected rename message, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "select-pane") || !strings.Contains(args, "-T") {
		t.Errorf("expected select-pane -T in tmux args, got: %s", args)
	}
	if !strings.Contains(args, "my-task") {
		t.Errorf("expected title in tmux args, got: %s", args)
	}
}

func TestRunRename_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runRename([]string{"%5"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

// --- broadcast subcommand tests ---

func TestRunBroadcast(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
case "$1" in
  list-panes)
    printf "%%3\tclaude\t12345\n%%5\tcodex\t12346\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runBroadcast([]string{"go", "test", "./..."}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Sent to pane %3") {
		t.Errorf("expected sent to %%3, got: %s", output)
	}
	if !strings.Contains(output, "Sent to pane %5") {
		t.Errorf("expected sent to %%5, got: %s", output)
	}
}

func TestRunBroadcast_NoPanes(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  list-panes)
    printf "%%1\tbash\t11111\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runBroadcast([]string{"hello"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No coding agent panes found") {
		t.Errorf("expected no panes message, got: %s", buf.String())
	}
}

func TestRunBroadcast_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runBroadcast(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing text")
	}
}

// --- kill-all subcommand tests ---

func TestRunKillAll(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
case "$1" in
  list-panes)
    printf "%%3\tclaude\t12345\n%%5\tcodex\t12346\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runKillAll(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Killed pane %3") {
		t.Errorf("expected killed %%3, got: %s", output)
	}
	if !strings.Contains(output, "Killed pane %5") {
		t.Errorf("expected killed %%5, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	if strings.Count(string(data), "kill-pane") != 2 {
		t.Errorf("expected 2 kill-pane calls, got: %s", string(data))
	}
}

func TestRunKillAll_NoPanes(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  list-panes)
    printf "%%1\tbash\t11111\n"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runKillAll(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No coding agent panes found") {
		t.Errorf("expected no panes message, got: %s", buf.String())
	}
}

// --- restart subcommand tests ---

func TestRunRestart(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	origDelay := restartDelay
	restartDelay = 0
	defer func() { restartDelay = origDelay }()

	var buf bytes.Buffer
	err := runRestart([]string{"%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Restarted session in pane %5") {
		t.Errorf("expected restart message, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "C-c") {
		t.Errorf("expected C-c in tmux args, got: %s", args)
	}
	if !strings.Contains(args, "/exit") {
		t.Errorf("expected /exit in tmux args, got: %s", args)
	}
	if !strings.Contains(args, "claude") {
		t.Errorf("expected claude in tmux args, got: %s", args)
	}
}

func TestRunRestart_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runRestart(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing pane ID")
	}
}

// --- history subcommand tests ---

func TestRunHistory(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
echo "history output line 1"
echo "history output line 2"
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runHistory([]string{"%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "history output") {
		t.Errorf("expected history output, got: %s", output)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	if !strings.Contains(string(data), "-1000") {
		t.Errorf("expected -1000 in tmux args, got: %s", string(data))
	}
}

func TestRunHistory_CustomLines(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
echo "output"
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runHistory([]string{"%5", "--lines", "500"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	if !strings.Contains(string(data), "-500") {
		t.Errorf("expected -500 in tmux args, got: %s", string(data))
	}
}

func TestRunHistory_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runHistory(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing pane ID")
	}
}

// --- diff subcommand tests ---

func TestRunDiff(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  capture-pane)
    case "$4" in
      %3) echo "output from pane 3" ;;
      %5) echo "output from pane 5" ;;
      *) echo "unknown pane" ;;
    esac
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runDiff([]string{"%3", "%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "=== Pane %3 ===") {
		t.Errorf("expected pane 3 header, got: %s", output)
	}
	if !strings.Contains(output, "=== Pane %5 ===") {
		t.Errorf("expected pane 5 header, got: %s", output)
	}
	if !strings.Contains(output, "output from pane 3") {
		t.Errorf("expected pane 3 output, got: %s", output)
	}
	if !strings.Contains(output, "output from pane 5") {
		t.Errorf("expected pane 5 output, got: %s", output)
	}
}

func TestRunDiff_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runDiff([]string{"%3"}, &buf)
	if err == nil {
		t.Fatal("expected error for missing second pane")
	}
}

// --- logs subcommand tests ---

func TestRunLogs(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  capture-pane)
    echo "log line 1"
    echo "log line 2"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	logFile := filepath.Join(dir, "test.log")
	var buf bytes.Buffer
	err := runLogs([]string{"%5", "--file", logFile}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Saved pane %5 output") {
		t.Errorf("expected saved message, got: %s", output)
	}
	if !strings.Contains(output, logFile) {
		t.Errorf("expected file path in output, got: %s", output)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if !strings.Contains(string(data), "log line 1") {
		t.Errorf("expected log content in file, got: %s", string(data))
	}
}

func TestRunLogs_DefaultPath(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "output"
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	var buf bytes.Buffer
	err := runLogs([]string{"%5"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Saved pane %5") {
		t.Errorf("expected saved message, got: %s", output)
	}
	if !strings.Contains(output, ".config/tmux-agent/logs/") {
		t.Errorf("expected default log path, got: %s", output)
	}
}

func TestRunLogs_MissingArgs(t *testing.T) {
	var buf bytes.Buffer
	err := runLogs(nil, &buf)
	if err == nil {
		t.Fatal("expected error for missing pane ID")
	}
}

// --- watch subcommand tests ---

func TestRunWatch_DispatcherRegistered(t *testing.T) {
	// Verify "watch" is recognized by the dispatcher (not "unknown command")
	// We can't actually run watch since it blocks, but we can verify it's registered.
	err := runSubcommand([]string{"watch", "--scan", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid --scan value")
	}
	if strings.Contains(err.Error(), "unknown command") {
		t.Error("watch should be a recognized command")
	}
}

// --- subcommand dispatcher tests ---

func TestRunSubcommand_UnknownCommand(t *testing.T) {
	err := runSubcommand([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected unknown command error, got: %v", err)
	}
}

func TestRunSubcommand_NoArgs(t *testing.T) {
	err := runSubcommand(nil)
	if err == nil {
		t.Fatal("expected error for no args")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Errorf("expected usage message, got: %v", err)
	}
}
