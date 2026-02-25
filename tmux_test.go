package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePaneList(t *testing.T) {
	input := "%3\tclaude\t12345\n%5\tnode\t12346\n%8\tbash\t12347\n%10\tcodex\t12348\n"

	panes := parsePaneList(input)

	// node is not a target command; only claude and codex should match
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes (claude, codex), got %d: %+v", len(panes), panes)
	}

	expected := map[string]string{
		"%3":  "claude",
		"%10": "codex",
	}
	for _, p := range panes {
		if cmd, ok := expected[p.ID]; !ok || cmd != p.Command {
			t.Errorf("unexpected pane: %+v", p)
		}
	}
}

func TestFindTargetChild(t *testing.T) {
	tests := []struct {
		name    string
		psOut   string
		panePID string
		want    string
	}{
		{
			name:    "claude as direct child",
			psOut:   "16174 14460 claude\n",
			panePID: "14460",
			want:    "claude",
		},
		{
			name:    "codex as direct child",
			psOut:   "16174 14460 codex\n",
			panePID: "14460",
			want:    "codex",
		},
		{
			name:    "no target child",
			psOut:   "16174 14460 vim\n",
			panePID: "14460",
			want:    "",
		},
		{
			name:    "node only is not a target",
			psOut:   "16174 14460 node\n",
			panePID: "14460",
			want:    "",
		},
		{
			name:    "multiple children one is claude",
			psOut:   "16174 14460 fish\n16175 14460 claude\n",
			panePID: "14460",
			want:    "claude",
		},
		{
			name:    "empty output",
			psOut:   "",
			panePID: "14460",
			want:    "",
		},
		{
			name:    "codex as grandchild via node",
			psOut:   "42545 14460 node\n42546 42545 codex\n",
			panePID: "14460",
			want:    "codex",
		},
		{
			name:    "claude as grandchild via shell",
			psOut:   "100 14460 bash\n200 100 claude\n",
			panePID: "14460",
			want:    "claude",
		},
		{
			name:    "codex with full path",
			psOut:   "42546 14460 /opt/homebrew/lib/node_modules/@openai/codex/codex\n",
			panePID: "14460",
			want:    "codex",
		},
		{
			name:    "node dev server is not a target",
			psOut:   "22535 14460 npm\n22564 22535 node\n",
			panePID: "14460",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findTargetChild(tt.psOut, tt.panePID)
			if got != tt.want {
				t.Errorf("findTargetChild() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePaneListWithChildProcess(t *testing.T) {
	input := "%3\tfish\t14460\n%5\tclaude\t12346\n%8\tbash\t12347\n"

	origFn := childLookupFn
	childLookupFn = func(panePID string) string {
		if panePID == "14460" {
			return "claude"
		}
		return ""
	}
	defer func() { childLookupFn = origFn }()

	panes := parsePaneList(input)

	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d: %+v", len(panes), panes)
	}

	found := map[string]bool{}
	for _, p := range panes {
		found[p.ID] = true
		if p.ID == "%3" && p.Command != "claude" {
			t.Errorf("expected command 'claude' for %%3, got %q", p.Command)
		}
	}
	if !found["%3"] || !found["%5"] {
		t.Errorf("expected %%3 and %%5, got %+v", panes)
	}
}

func TestDetectIdle(t *testing.T) {
	now := time.Now()

	t.Run("idle", func(t *testing.T) {
		p := &paneInfo{LastChangeAt: now.Add(-15 * time.Minute)}
		if !detectIdle(p, 10*time.Minute) {
			t.Error("expected pane to be idle")
		}
	})

	t.Run("active", func(t *testing.T) {
		p := &paneInfo{LastChangeAt: now.Add(-5 * time.Minute)}
		if detectIdle(p, 10*time.Minute) {
			t.Error("expected pane to be active")
		}
	})

	t.Run("exactly at threshold", func(t *testing.T) {
		p := &paneInfo{LastChangeAt: now.Add(-10 * time.Minute)}
		if !detectIdle(p, 10*time.Minute) {
			t.Error("expected pane at threshold to be idle")
		}
	})
}

func TestStatusShort(t *testing.T) {
	panes := []paneInfo{
		{ID: "%1", Command: "claude", LastChangeAt: time.Now()},
		{ID: "%2", Command: "claude", LastChangeAt: time.Now()},
		{ID: "%3", Command: "codex", LastChangeAt: time.Now().Add(-20 * time.Minute)},
		{ID: "%4", Command: "codex", LastChangeAt: time.Now()},
	}

	got := statusShort(panes, 10*time.Minute)
	if !strings.Contains(got, "3 active") {
		t.Errorf("expected '3 active', got: %s", got)
	}
	if !strings.Contains(got, "1 idle") {
		t.Errorf("expected '1 idle', got: %s", got)
	}
}

func TestSendTmuxKeysEmptyInput(t *testing.T) {
	tests := []struct {
		name string
		keys string
	}{
		{"empty string", ""},
		{"newline only", "\n"},
		{"multiple newlines", "\n\n\n"},
		{"carriage return", "\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sendTmuxKeys("%3", tt.keys)
			if err != nil {
				t.Errorf("expected no error for empty input, got: %v", err)
			}
		})
	}
}

func TestSendTmuxKeysUsesSendKeysLiteral(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" >> `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	err := sendTmuxKeys("%3", "go test ./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed to read args file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 tmux invocations, got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "send-keys") || !strings.Contains(lines[0], "-l") {
		t.Errorf("first call should be send-keys -l, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "go test ./...") {
		t.Errorf("expected text in send-keys -l, got: %s", lines[0])
	}

	if !strings.Contains(lines[1], "send-keys") || !strings.Contains(lines[1], "C-m") {
		t.Errorf("second call should be send-keys C-m, got: %s", lines[1])
	}

	if !strings.Contains(lines[2], "send-keys") || !strings.Contains(lines[2], "C-m") {
		t.Errorf("third call should be send-keys C-m, got: %s", lines[2])
	}
}

func TestSendTmuxKeysSpecialChars(t *testing.T) {
	dir := t.TempDir()

	contentFile := filepath.Join(dir, "content.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
if echo "$@" | grep -q "\-l"; then
  shift; shift; shift; shift; shift
  printf '%s' "$*" > `+contentFile+`
fi
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	specialKeys := `echo "hello $USER" && echo '$(whoami)' | grep "test"`
	err := sendTmuxKeys("%5", specialKeys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(contentFile)
	if err != nil {
		t.Fatalf("failed to read content file: %v", err)
	}
	if string(data) != specialKeys {
		t.Errorf("expected special chars to be preserved.\ngot:  %q\nwant: %q", string(data), specialKeys)
	}
}

func TestSendTmuxKeysCollapsesNewlines(t *testing.T) {
	dir := t.TempDir()

	contentFile := filepath.Join(dir, "content.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
if echo "$@" | grep -q "\-l"; then
  shift; shift; shift; shift; shift
  printf '%s' "$*" > `+contentFile+`
fi
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	err := sendTmuxKeys("%3", "line1\nline2\nline3\n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(contentFile)
	if err != nil {
		t.Fatalf("failed to read content file: %v", err)
	}
	if string(data) != "line1 line2 line3" {
		t.Errorf("expected newlines collapsed to spaces, got: %q", string(data))
	}
}

func TestSendTmuxKeysStripsTrailingCm(t *testing.T) {
	dir := t.TempDir()

	contentFile := filepath.Join(dir, "content.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
if echo "$@" | grep -q "\-l"; then
  shift; shift; shift; shift; shift
  printf '%s' "$*" > `+contentFile+`
fi
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	tests := []struct {
		name string
		keys string
		want string
	}{
		{"trailing C-m C-m", "go test ./... C-m C-m", "go test ./..."},
		{"trailing C-m", "hello C-m", "hello"},
		{"trailing Enter", "hello Enter", "hello"},
		{"trailing \\n", `hello\n`, "hello"},
		{"no trailing", "hello world", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Remove(contentFile)
			err := sendTmuxKeys("%3", tt.keys)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			data, err := os.ReadFile(contentFile)
			if err != nil {
				t.Fatalf("failed to read content: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("got %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestKillTmuxPane(t *testing.T) {
	dir := t.TempDir()

	argsFile := filepath.Join(dir, "tmux-args.txt")
	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
echo "$@" > `+argsFile+`
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	err := killTmuxPane("%5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("tmux was not called: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "kill-pane") {
		t.Errorf("expected kill-pane in tmux args, got: %s", args)
	}
	if !strings.Contains(args, "%5") {
		t.Errorf("expected pane ID in tmux args, got: %s", args)
	}
}

func TestCreateTmuxPane(t *testing.T) {
	dir := t.TempDir()

	tmuxScript := filepath.Join(dir, "tmux")
	os.WriteFile(tmuxScript, []byte(`#!/bin/sh
case "$1" in
  split-window)
    echo "%99"
    ;;
esac
`), 0755)

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	paneID, err := createTmuxPane("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paneID != "%99" {
		t.Errorf("expected pane ID %%99, got %q", paneID)
	}
}
