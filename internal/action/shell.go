package action

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type ShellAction struct {
	Script string `yaml:"script"`
}

func (a *ShellAction) String() string {
	return fmt.Sprintf("shell(script=%s)", a.Script)
}

func (a *ShellAction) Act(ctx *ActionContext) {
	// FIXME: Validate file on config load
	cmd := exec.Command(a.Script)
	env := os.Environ()
	env = append(env, fmt.Sprintf("PAVLOV_RULE=%s", ctx.Rule))
	env = append(env, fmt.Sprintf("PAVLOV_FILE=%s", ctx.File))
	env = append(env, fmt.Sprintf("PAVLOV_LINE=%s", ctx.Line))
	env = append(env, fmt.Sprintf("PAVLOV_TS=%d", ctx.Timestamp.Unix()))
	env = append(env, fmt.Sprintf("PAVLOV_GROUP=%s", ctx.Group))
	env = append(env, fmt.Sprintf("PAVLOV_GROUP_BY=%s", ctx.GroupBy))
	for k, v := range ctx.Vars {
		env = append(env, fmt.Sprintf("PAVLOV_VAR_%s=%s", k, v))
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if err != nil {
		slog.Error(
			"failed to execute shell command",
			"rule", ctx.Rule,
			"script", a.Script,
			"err", err,
			"exit_code", cmd.ProcessState.ExitCode(),
			"stdout", stdout.String(),
			"stderr", stderr.String(),
		)
		return
	}
	slog.Debug("shell command executed", "rule", ctx.Rule, "script", a.Script)
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
	return nil
}
