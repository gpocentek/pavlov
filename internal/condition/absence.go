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
	// AbsenceTick is set when the time-base check runs
	if !ctx.AbsenceTick {
		ctx.State.LastSeen = ctx.Timestamp
		return false
	}

	absence := time.Duration(c.Duration) * time.Second
	if ctx.Timestamp.Sub(ctx.State.LastSeen) <= absence {
		return false
	}

	return true
}

func (c *AbsenceCondition) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("`duration` must be defined and greater than 0")
	}
	return nil
}
