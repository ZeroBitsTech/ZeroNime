package kuramanime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (p *Provider) renderPlayerHTML(ctx context.Context, targetURL string) (string, error) {
	if strings.TrimSpace(p.browserPath) == "" {
		return "", fmt.Errorf("browser renderer unavailable")
	}

	profileDir, err := os.MkdirTemp("", "zeronime-kuramanime-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profileDir)

	args := []string{
		"--headless",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--disable-background-networking",
		"--mute-audio",
		"--no-first-run",
		"--no-default-browser-check",
		"--no-sandbox",
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		fmt.Sprintf("--virtual-time-budget=%d", p.browserRenderBudget.Milliseconds()),
		"--dump-dom",
		targetURL,
	}

	cmd := exec.CommandContext(ctx, p.browserPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("render player html: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("render player html: %w", err)
	}
	return string(output), nil
}
