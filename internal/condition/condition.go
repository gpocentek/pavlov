package condition

import (
	"time"
)

type GroupState struct {
	Window    []time.Time
	LastFired time.Time
	LastSeen  time.Time
}

type ConditionContext struct {
	Line        string
	Vars        map[string]string
	Group       string
	Cooldown    uint
	Timestamp   time.Time
	State       *GroupState
	AbsenceTick bool
}

type Condition interface {
	Eval(*ConditionContext) bool
	Validate() error
}
