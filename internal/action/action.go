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
	Captures  map[string]string
}

type RunOptions struct {
	Timeout      *uint `yaml:"timeout"`
	StopPrevious *bool `yaml:"stop_previous"`
}

type Action interface {
	RunOptions() RunOptions
	Act(context.Context, *ActionContext)
	Validate() error
}

func setDefaultRunOptions(o *RunOptions) {
	if o.Timeout == nil {
		d := uint(0)
		o.Timeout = &d
	}
	if o.StopPrevious == nil {
		b := false
		o.StopPrevious = &b
	}
}
