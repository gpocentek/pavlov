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
	ctx.State.Window = append(ctx.State.Window, ctx.Timestamp)
	ctx.State.LastSeen = ctx.Timestamp

	cutoff := ctx.Timestamp.Add(-time.Duration(c.Window) * time.Second)
	pruned := ctx.State.Window[:0]
	for _, t := range ctx.State.Window {
		if !t.Before(cutoff) {
			pruned = append(pruned, t)
		}
	}
	ctx.State.Window = pruned

	return len(ctx.State.Window) >= int(c.Threshold)
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
