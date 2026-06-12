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
	timestamp := ctx.Timestamp
	if !ctx.AbsenceTick {
		ctx.State.LastSeen = timestamp
		return false
	}

	absence := time.Duration(c.Duration) * time.Second
	if timestamp.Sub(ctx.State.LastSeen) <= absence {
		return false
	}

	cooldown := time.Duration(ctx.Cooldown) * time.Second
	if !ctx.State.LastFired.IsZero() && timestamp.Sub(ctx.State.LastFired) <= cooldown {
		return false
	}

	ctx.State.LastFired = timestamp
	return true
}

func (c *AbsenceCondition) Validate() error {
	if c.Duration <= 0 {
		return fmt.Errorf("`duration` must be defined and greater than 0")
	}
	return nil
}
