package action

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"
)

type LogAction struct {
	Options  RunOptions `yaml:",inline"`
	Template string     `yaml:"template"`
	template *template.Template
}

func (a *LogAction) RunOptions() RunOptions {
	return a.Options
}

func (a *LogAction) String() string {
	return fmt.Sprintf(
		"log(template=%s, timeout=%d, stop_previous=%t)",
		a.Template, *a.Options.Timeout, *a.Options.StopPrevious,
	)
}

func (a *LogAction) Act(ctx context.Context, actionCtx *ActionContext) {
	buf := bytes.NewBufferString("")
	err := a.template.Execute(buf, actionCtx)
	if err != nil {
		slog.Error("failed to execute log template", "rule", actionCtx.Rule, "err", err)
		return
	}
	slog.Info(buf.String(), "rule", actionCtx.Rule, "file", actionCtx.File)
}

func (a *LogAction) Validate() error {
	tmpl, err := template.New("log").Parse(a.Template)
	if err != nil {
		return fmt.Errorf("failed to parse log template: %w", err)
	}
	a.template = tmpl

	setDefaultRunOptions(&a.Options)

	return nil
}
