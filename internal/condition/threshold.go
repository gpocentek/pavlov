package condition

import (
	"fmt"
	"time"
)

type ThresholdCondition struct {
	Count  uint `yaml:"count"`
	Window uint `yaml:"window"`
}

func (c *ThresholdCondition) String() string {
	return fmt.Sprintf("threshold(count=%d, window=%d)", c.Count, c.Window)
}

func (c *ThresholdCondition) Eval(ctx *ConditionContext) bool {
	ctx.State.MatchTimes = append(ctx.State.MatchTimes, ctx.Timestamp)

	cutoff := ctx.Timestamp.Add(-time.Duration(c.Window) * time.Second)
	pruned := ctx.State.MatchTimes[:0]
	for _, t := range ctx.State.MatchTimes {
		if !t.Before(cutoff) {
			pruned = append(pruned, t)
		}
	}
	ctx.State.MatchTimes = pruned

	return len(ctx.State.MatchTimes) >= int(c.Count)
}

func (c *ThresholdCondition) Validate() error {
	if c.Count < 1 {
		return fmt.Errorf("`count` must be greater than 0")
	}
	if c.Window < 1 {
		return fmt.Errorf("`window` must be greater than 0")
	}
	return nil
}
