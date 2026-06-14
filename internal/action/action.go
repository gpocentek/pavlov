package action

import (
	"context"
	"time"
)

type ActionContext struct {
	Rule      string
	File      string
	Line      string
	GroupBy   string
	Group     string
	Timestamp time.Time
	Vars      map[string]string
}

type ActionConfig struct {
	Timeout      *uint `yaml:"timeout"`
	StopPrevious *bool `yaml:"stop_previous"`
}

type Action interface {
	GetActionConfig() ActionConfig
	Act(context.Context, *ActionContext)
	Validate() error
}

func setDefaultActionConfigValues(a *ActionConfig) {
	if a.Timeout == nil {
		d := uint(0)
		a.Timeout = &d
	}
	if a.StopPrevious == nil {
		b := false
		a.StopPrevious = &b
	}
}
