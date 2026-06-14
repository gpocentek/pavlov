package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
)

type ShellAction struct {
	Options RunOptions `yaml:",inline"`
	Script  string     `yaml:"script"`
}

func (a *ShellAction) RunOptions() RunOptions {
	return a.Options
}

func (a *ShellAction) String() string {
	return fmt.Sprintf(
		"shell(script=%s, timeout=%d, stop_previous=%t)",
		a.Script, *a.Options.Timeout, *a.Options.StopPrevious)
}

func (a *ShellAction) Act(ctx context.Context, actionCtx *ActionContext) {
	cmd := exec.CommandContext(ctx, a.Script)
	// CommandContext kills only the direct child (the shell running the script).
	// Child processes spawned by the script (e.g. sleep, curl) would otherwise
	// keep running as orphans. Setpgid starts the script in its own process
	// group; Kill(-pid) sends SIGKILL to that entire group on timeout/cancel.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// group PIDs are negative, hence the - prefix
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	env := os.Environ()
	env = append(env, fmt.Sprintf("PAVLOV_RULE=%s", actionCtx.Rule))
	env = append(env, fmt.Sprintf("PAVLOV_FILE=%s", actionCtx.File))
	env = append(env, fmt.Sprintf("PAVLOV_LINE=%s", actionCtx.Line))
	env = append(env, fmt.Sprintf("PAVLOV_TS=%d", actionCtx.Timestamp.Unix()))
	env = append(env, fmt.Sprintf("PAVLOV_GROUP=%s", actionCtx.Group))
	env = append(env, fmt.Sprintf("PAVLOV_GROUP_BY=%s", actionCtx.GroupBy))
	for k, v := range actionCtx.Captures {
		env = append(env, fmt.Sprintf("PAVLOV_CAPTURE_%s=%s", k, v))
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.Warn(
				"shell action timed out",
				"rule", actionCtx.Rule,
				"script", a.Script,
				"timeout", *a.Options.Timeout,
				"stdout", stdout.String(),
				"stderr", stderr.String(),
			)
			return
		}
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		slog.Error(
			"failed to execute shell command",
			"rule", actionCtx.Rule,
			"script", a.Script,
			"err", err,
			"exit_code", exitCode,
			"stdout", stdout.String(),
			"stderr", stderr.String(),
		)
		return
	}
	slog.Debug("shell command executed", "rule", actionCtx.Rule, "script", a.Script)
}

func (a *ShellAction) Validate() error {
	// Script is required
	if a.Script == "" {
		return fmt.Errorf("`script` is required")
	}

	// Script must exist
	info, err := os.Stat(a.Script)
	if os.IsNotExist(err) || err != nil {
		return fmt.Errorf("script %s does not exist", a.Script)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("script %s is not executable", a.Script)
	}

	setDefaultRunOptions(&a.Options)

	return nil
}
