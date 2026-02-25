package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const defaultAgentCommand = "claude"

// activeAgent is the resolved agent command for this invocation.
// Set at startup from config file, overridable with --claude/--codex flags.
var activeAgent = defaultAgentCommand

// agentConfig holds persisted settings.
type agentConfig struct {
	DefaultAgent string `json:"default_agent"`
}

// configDir returns the configuration directory path.
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tmux-agent")
}

// configFilePath returns the path to the config file.
func configFilePath() string {
	return filepath.Join(configDir(), "config.json")
}

// loadConfig reads the config file. Returns defaults if not found.
func loadConfig() *agentConfig {
	cfg := &agentConfig{DefaultAgent: defaultAgentCommand}
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	if cfg.DefaultAgent == "" {
		cfg.DefaultAgent = defaultAgentCommand
	}
	return cfg
}

// saveConfig writes the config file.
func saveConfig(cfg *agentConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), data, 0644)
}

// parseGlobalFlags extracts global flags (--claude, --codex, --set-default-agent)
// from args. Returns the remaining args and whether a config-only action was performed.
func parseGlobalFlags(args []string) (remaining []string, handled bool) {
	cfg := loadConfig()
	activeAgent = cfg.DefaultAgent

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--claude":
			activeAgent = "claude"
		case "--codex":
			activeAgent = "codex"
		case "--set-default-agent":
			if i+1 < len(args) {
				i++
				cfg.DefaultAgent = args[i]
				if err := saveConfig(cfg); err != nil {
					os.Stderr.WriteString("error: " + err.Error() + "\n")
					os.Exit(1)
				}
				os.Stdout.WriteString("Default agent set to " + cfg.DefaultAgent + "\n")
				return nil, true
			}
		default:
			remaining = append(remaining, args[i])
		}
	}
	return remaining, false
}
