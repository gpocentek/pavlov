package condition

import (
	"fmt"
	"time"
)

type ThresholdCondition struct {
	Threshold uint `yaml:"threshold"`
	Window    uint `yaml:"window"`
}

func (c *ThresholdCondition) String() string {
	return fmt.Sprintf("threshold(threshold=%d, window=%d)", c.Threshold, c.Window)
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

	return len(ctx.State.MatchTimes) >= int(c.Threshold)
}

func (c *ThresholdCondition) Validate() error {
	if c.Threshold < 1 {
		return fmt.Errorf("`threshold` must be greater than 0")
	}
	if c.Window < 1 {
		return fmt.Errorf("`window` must be greater than 0")
	}
	return nil
}
