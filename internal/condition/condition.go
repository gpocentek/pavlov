package condition

import "time"

type ConditionState struct {
	MatchTimes []time.Time
	LastSeen   time.Time
}

type ConditionContext struct {
	Line       string
	Captures   map[string]string
	GroupValue string
	Timestamp  time.Time
	State      *ConditionState
	FromTicker bool
}

type Condition interface {
	Eval(*ConditionContext) bool
	Validate() error
}
