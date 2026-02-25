# tmux-agent

Manage coding agent panes in tmux.

## Install

```
go install github.com/sat0b/tmux-agent@latest
```

## Usage

```
tmux-agent <command>

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
  watch [--scan duration] [--idle duration] [--log path]  Monitor panes

Workspace:
  workspace --repo <owner/repo> [--issue N] [--branch name]  Create worktree + pane
```

## Examples

```bash
# List active panes
tmux-agent panes

# Send a prompt to a pane
tmux-agent send %5 "run the tests and fix any failures"

# See what a pane is doing
tmux-agent capture %5 --lines 20

# Create a new pane and send an initial prompt
tmux-agent create --keys "review the open PRs"

# Create in a specific session as a new window
tmux-agent create --session work --new-window

# Vertical split
tmux-agent create --split v

# Use codex instead of the default agent
tmux-agent --codex create

# Change the default agent (persisted to ~/.config/tmux-agent/config.json)
tmux-agent --set-default-agent codex

# Send the same instruction to all panes
tmux-agent broadcast "commit your changes and report what you did"

# Set up a workspace from a GitHub issue (creates worktree + pane)
tmux-agent workspace --repo user/repo --issue 42

# Check status of all panes
tmux-agent status

# Monitor panes and log idle detection
tmux-agent watch --scan 5s --idle 5m

# Monitor with log file
tmux-agent watch --log /tmp/agent-watch.log
```

## License

MIT
