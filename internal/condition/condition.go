package condition

import (
	"context"
	"time"
)

type GroupState struct {
	Window    []time.Time
	LastFired time.Time
	LastSeen  time.Time
	// Cancel and RunCtx track the in-flight action for stop_previous.
	Cancel context.CancelFunc
	RunCtx context.Context
}

type ConditionContext struct {
	Line        string
	Vars        map[string]string
	Group       string
	Timestamp   time.Time
	State       *GroupState
	AbsenceTick bool
}

type Condition interface {
	Eval(*ConditionContext) bool
	Validate() error
}
