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
	// FromTicker is set when the periodic ticker runs the check.
	if !ctx.FromTicker {
		ctx.State.LastSeen = ctx.Timestamp
		return false
	}

	absence := time.Duration(c.Duration) * time.Second
	return ctx.Timestamp.Sub(ctx.State.LastSeen) > absence
}

func (c *AbsenceCondition) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("`duration` must be defined and greater than 0")
	}
	return nil
}
