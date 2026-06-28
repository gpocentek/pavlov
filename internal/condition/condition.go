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
}

type Condition interface {
	Eval(*ConditionContext) bool
	Validate() error
}

// PeriodicEvaluator is implemented by conditions that must be re-checked on a
// timer (e.g. absence), because they can become true without a new log line.
type PeriodicEvaluator interface {
	EvalPeriodic(*ConditionContext) bool
}

// InstanceSeeder is implemented by conditions that need initial per-scope state
// before any matching line is seen (e.g. absence without group_by).
type InstanceSeeder interface {
	SeedInstances(groupBy string) map[string]*ConditionState
}
