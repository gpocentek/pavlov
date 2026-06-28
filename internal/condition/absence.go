package condition

import (
	"fmt"
	"time"
)

type AbsenceCondition struct {
	Duration uint `yaml:"duration"`
}

func (c *AbsenceCondition) String() string {
	return fmt.Sprintf("absence(duration=%d)", c.Duration)
}

func (c *AbsenceCondition) Eval(ctx *ConditionContext) bool {
	ctx.State.LastSeen = ctx.Timestamp
	return false
}

func (c *AbsenceCondition) EvalPeriodic(ctx *ConditionContext) bool {
	absence := time.Duration(c.Duration) * time.Second
	return ctx.Timestamp.Sub(ctx.State.LastSeen) > absence
}

// SeedInstances returns initial per-scope state for periodic evaluation. When
// group_by is not set, the default scope ("") starts with LastSeen set to now so
// absence can fire even if no matching line has ever been seen. When group_by is
// set, scopes are created lazily on the first matching line per group.
func (c *AbsenceCondition) SeedInstances(groupBy string) map[string]*ConditionState {
	if groupBy != "" {
		return nil
	}
	return map[string]*ConditionState{
		"": {LastSeen: time.Now()},
	}
}

func (c *AbsenceCondition) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("`duration` must be defined and greater than 0")
	}
	return nil
}
