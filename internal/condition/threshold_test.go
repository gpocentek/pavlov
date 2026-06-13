package condition

import (
	"fmt"
	"testing"
	"time"
)

func TestThresholdCondition(t *testing.T) {
	threshold := 5
	window := 60
	condition := &ThresholdCondition{Threshold: uint(threshold), Window: uint(window)}
	err := condition.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	conditionStr := fmt.Sprintf("threshold(threshold=%d, window=%d)", threshold, window)
	if condition.String() != conditionStr {
		t.Fatalf("expected '%s', got %s", conditionStr, condition.String())
	}
}

func TestThresholdConditionValidateFailThreshold(t *testing.T) {
	condition := &ThresholdCondition{Threshold: 0, Window: 60}
	err := condition.Validate()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "`threshold` must be greater than 0" {
		t.Fatalf("expected '`threshold` must be greater than 0', got %s", err.Error())
	}
}

func TestThresholdConditionValidateFailWindow(t *testing.T) {
	condition := &ThresholdCondition{Threshold: 5, Window: 0}
	err := condition.Validate()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "`window` must be greater than 0" {
		t.Fatalf("expected '`window` must be greater than 0', got %s", err.Error())
	}
}

func TestThresholdConditionEvalNotMet(t *testing.T) {
	condition := &ThresholdCondition{Threshold: 3, Window: 60}
	now := time.Now()
	ctx := &ConditionContext{
		Timestamp: now,
		State:     &GroupState{},
	}

	if got := condition.Eval(ctx); got != false {
		t.Fatalf("expected false, got %v", got)
	}
	if ctx.State.LastSeen != now {
		t.Fatalf("expected last seen to be %v, got %v", now, ctx.State.LastSeen)
	}
	if len(ctx.State.Window) != 1 {
		t.Fatalf("expected window length 1, got %d", len(ctx.State.Window))
	}
}

func TestThresholdConditionEvalMet(t *testing.T) {
	condition := &ThresholdCondition{Threshold: 3, Window: 60}
	now := time.Now()
	ctx := &ConditionContext{
		State: &GroupState{},
	}

	for i := range 2 {
		ctx.Timestamp = now.Add(time.Duration(i) * time.Second)
		if got := condition.Eval(ctx); got != false {
			t.Fatalf("event %d: expected false, got %v", i+1, got)
		}
	}

	ctx.Timestamp = now.Add(2 * time.Second)
	if got := condition.Eval(ctx); got != true {
		t.Fatalf("expected true, got %v", got)
	}
	if ctx.State.LastSeen != ctx.Timestamp {
		t.Fatalf("expected last seen to be %v, got %v", ctx.Timestamp, ctx.State.LastSeen)
	}
	if len(ctx.State.Window) != 3 {
		t.Fatalf("expected window length 3, got %d", len(ctx.State.Window))
	}
}

func TestThresholdConditionEvalPrunesOutsideWindow(t *testing.T) {
	condition := &ThresholdCondition{Threshold: 2, Window: 10}
	now := time.Now()
	ctx := &ConditionContext{
		State: &GroupState{},
	}

	ctx.Timestamp = now
	if got := condition.Eval(ctx); got != false {
		t.Fatalf("first event: expected false, got %v", got)
	}

	ctx.Timestamp = now.Add(15 * time.Second)
	if got := condition.Eval(ctx); got != false {
		t.Fatalf("second event after window: expected false, got %v", got)
	}
	if len(ctx.State.Window) != 1 {
		t.Fatalf("expected pruned window length 1, got %d", len(ctx.State.Window))
	}

	ctx.Timestamp = now.Add(16 * time.Second)
	if got := condition.Eval(ctx); got != true {
		t.Fatalf("third event within window: expected true, got %v", got)
	}
	if len(ctx.State.Window) != 2 {
		t.Fatalf("expected final window length 2, got %d", len(ctx.State.Window))
	}
}
