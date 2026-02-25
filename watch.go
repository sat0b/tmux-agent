package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const defaultScanInterval = 10 * time.Second

// runWatch monitors tmux panes and logs idle detection.
func runWatch(args []string) error {
	scanInterval := defaultScanInterval
	idleThreshold := defaultIdleThreshold
	logFile := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--scan":
			if i+1 < len(args) {
				i++
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return fmt.Errorf("invalid --scan value: %s", args[i])
				}
				scanInterval = d
			}
		case "--idle":
			if i+1 < len(args) {
				i++
				d, err := time.ParseDuration(args[i])
				if err != nil {
					return fmt.Errorf("invalid --idle value: %s", args[i])
				}
				idleThreshold = d
			}
		case "--log":
			if i+1 < len(args) {
				i++
				logFile = args[i]
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var writers []io.Writer
	writers = append(writers, os.Stdout)
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer f.Close()
		writers = append(writers, f)
	}

	logger := log.New(io.MultiWriter(writers...), "[tmux-agent:watch] ", log.LstdFlags)

	paneOutputs := make(map[string]string)
	paneLastChange := make(map[string]time.Time)

	scanTicker := time.NewTicker(scanInterval)
	defer scanTicker.Stop()

	logger.Printf("watching tmux panes (scan: %s, idle threshold: %s)", scanInterval, idleThreshold)

	for {
		select {
		case <-scanTicker.C:
			panes, err := listTmuxPanes()
			if err != nil {
				logger.Printf("[warn] failed to list panes: %v", err)
				continue
			}

			for i := range panes {
				output, err := capturePaneOutput(panes[i].ID, 10)
				if err != nil {
					continue
				}

				prev, exists := paneOutputs[panes[i].ID]
				if !exists || prev != output {
					paneOutputs[panes[i].ID] = output
					paneLastChange[panes[i].ID] = time.Now()
				}

				if lastChange, ok := paneLastChange[panes[i].ID]; ok {
					panes[i].LastChangeAt = lastChange
					panes[i].LastOutput = output
				}

				if detectIdle(&panes[i], idleThreshold) {
					logger.Printf("[idle] pane %s (%s) idle for %s",
						panes[i].ID, panes[i].Command,
						time.Since(panes[i].LastChangeAt).Truncate(time.Second))
				}
			}

		case sig := <-sigCh:
			logger.Printf("received %s, shutting down", sig)
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}
