package action

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writeExecutableScript(t *testing.T, body string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return script
}

func sleepProcessRunning(marker string) bool {
	out, err := exec.Command("pgrep", "-f", marker).Output()
	return err == nil && len(out) > 0
}

func TestShellActionTimeoutKillsProcessGroup(t *testing.T) {
	const sleepMarker = "pavlov-shell-timeout-test-sleep-99999"
	script := writeExecutableScript(t, "#!/bin/sh\nsleep 99999 # "+sleepMarker+"\n")

	timeout := uint(1)
	stopPrevious := false
	a := &ShellAction{
		Script: script,
		ActionConfig: ActionConfig{
			Timeout:      &timeout,
			StopPrevious: &stopPrevious,
		},
	}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()
	a.Act(ctx, &ActionContext{Rule: "test_rule"})
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("expected action to return after ~1s timeout, took %v", elapsed)
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", ctx.Err())
	}
	if sleepProcessRunning(sleepMarker) {
		t.Fatal("expected sleep child to be killed with the process group")
	}
}
