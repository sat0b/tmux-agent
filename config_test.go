package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	cfg := loadConfig()
	if cfg.DefaultAgent != "claude" {
		t.Errorf("expected default agent 'claude', got %q", cfg.DefaultAgent)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	cfg := &agentConfig{DefaultAgent: "codex"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	loaded := loadConfig()
	if loaded.DefaultAgent != "codex" {
		t.Errorf("expected 'codex', got %q", loaded.DefaultAgent)
	}

	// Verify file exists
	path := filepath.Join(dir, ".config", "tmux-agent", "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not found: %v", err)
	}
}

func TestParseGlobalFlags_Claude(t *testing.T) {
	activeAgent = defaultAgentCommand
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	remaining, handled := parseGlobalFlags([]string{"--claude", "panes"})
	if handled {
		t.Fatal("expected handled=false")
	}
	if activeAgent != "claude" {
		t.Errorf("expected agent 'claude', got %q", activeAgent)
	}
	if len(remaining) != 1 || remaining[0] != "panes" {
		t.Errorf("unexpected remaining args: %v", remaining)
	}
}

func TestParseGlobalFlags_Codex(t *testing.T) {
	activeAgent = defaultAgentCommand
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	remaining, handled := parseGlobalFlags([]string{"--codex", "create"})
	if handled {
		t.Fatal("expected handled=false")
	}
	if activeAgent != "codex" {
		t.Errorf("expected agent 'codex', got %q", activeAgent)
	}
	if len(remaining) != 1 || remaining[0] != "create" {
		t.Errorf("unexpected remaining args: %v", remaining)
	}
}

func TestParseGlobalFlags_DefaultFromConfig(t *testing.T) {
	activeAgent = defaultAgentCommand
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// Save config with codex as default
	saveConfig(&agentConfig{DefaultAgent: "codex"})

	remaining, handled := parseGlobalFlags([]string{"panes"})
	if handled {
		t.Fatal("expected handled=false")
	}
	if activeAgent != "codex" {
		t.Errorf("expected agent 'codex' from config, got %q", activeAgent)
	}
	if len(remaining) != 1 || remaining[0] != "panes" {
		t.Errorf("unexpected remaining args: %v", remaining)
	}
}

func TestParseGlobalFlags_FlagOverridesConfig(t *testing.T) {
	activeAgent = defaultAgentCommand
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// Config says codex, but --claude flag overrides
	saveConfig(&agentConfig{DefaultAgent: "codex"})

	remaining, handled := parseGlobalFlags([]string{"--claude", "create"})
	if handled {
		t.Fatal("expected handled=false")
	}
	if activeAgent != "claude" {
		t.Errorf("expected agent 'claude' (flag override), got %q", activeAgent)
	}
	if len(remaining) != 1 || remaining[0] != "create" {
		t.Errorf("unexpected remaining args: %v", remaining)
	}
}
