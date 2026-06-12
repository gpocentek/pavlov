package condition

import (
	"time"
)

type SeenCondition struct {
}

func (c *SeenCondition) String() string {
	return "seen()"
}

func (c *SeenCondition) Eval(ctx *ConditionContext) bool {
	timestamp := ctx.Timestamp

	cooldown := time.Duration(ctx.Cooldown) * time.Second
	if !ctx.State.LastFired.IsZero() && timestamp.Sub(ctx.State.LastFired) <= cooldown {
		return false
	}

	ctx.State.LastFired = timestamp
	return true
}

func (c *SeenCondition) Validate() error {
	return nil
}
