package action

import (
	"bytes"
	"fmt"
	"log/slog"
	"text/template"
)

type LogAction struct {
	File     string `yaml:"file"`
	Template string `yaml:"template"`
	template *template.Template
}

func (a *LogAction) String() string {
	return fmt.Sprintf("log(file=%s, format=%s)", a.File, a.Template)
}

func (a *LogAction) Act(ctx *ActionContext) {
	buf := bytes.NewBufferString("")
	err := a.template.Execute(buf, ctx)
	if err != nil {
		slog.Error("failed to execute log template", "rule", ctx.Rule, "err", err)
		return
	}
	slog.Info(buf.String(), "rule", ctx.Rule, "file", ctx.File)
}

func (a *LogAction) Validate() error {
	tmpl, err := template.New("log").Parse(a.Template)
	if err != nil {
		return fmt.Errorf("failed to parse log template: %w", err)
	}
	a.template = tmpl
	return nil
}
