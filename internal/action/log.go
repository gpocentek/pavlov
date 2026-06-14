package action

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"
)

type LogAction struct {
	ActionConfig `yaml:",inline"`
	File         string `yaml:"file"`
	Template     string `yaml:"template"`
	template     *template.Template
}

func (a *LogAction) GetActionConfig() ActionConfig {
	return a.ActionConfig
}

func (a *LogAction) String() string {
	return fmt.Sprintf(
		"log(file=%s, format=%s, timeout=%d, stop_previous=%t)",
		a.File, a.Template, *a.Timeout, *a.StopPrevious,
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

	setDefaultActionConfigValues(&a.ActionConfig)

	return nil
}
