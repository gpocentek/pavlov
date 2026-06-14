package condition

import (
	"fmt"
	"testing"
	"time"
)

func TestThresholdCondition(t *testing.T) {
	count := 5
	window := 60
	condition := &ThresholdCondition{Count: uint(count), Window: uint(window)}
	err := condition.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	conditionStr := fmt.Sprintf("threshold(count=%d, window=%d)", count, window)
	if condition.String() != conditionStr {
		t.Fatalf("expected '%s', got %s", conditionStr, condition.String())
	}
}

func TestThresholdConditionValidateFailCount(t *testing.T) {
	condition := &ThresholdCondition{Count: 0, Window: 60}
	err := condition.Validate()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "`count` must be greater than 0" {
		t.Fatalf("expected '`count` must be greater than 0', got %s", err.Error())
	}
}

func TestThresholdConditionValidateFailWindow(t *testing.T) {
	condition := &ThresholdCondition{Count: 5, Window: 0}
	err := condition.Validate()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "`window` must be greater than 0" {
		t.Fatalf("expected '`window` must be greater than 0', got %s", err.Error())
	}
}

func TestThresholdConditionEvalNotMet(t *testing.T) {
	condition := &ThresholdCondition{Count: 3, Window: 60}
	now := time.Now()
	ctx := &ConditionContext{
		Timestamp: now,
		State:     &ConditionState{},
	}

	if got := condition.Eval(ctx); got != false {
		t.Fatalf("expected false, got %v", got)
	}
	if len(ctx.State.MatchTimes) != 1 {
		t.Fatalf("expected match times length 1, got %d", len(ctx.State.MatchTimes))
	}
}

func TestThresholdConditionEvalMet(t *testing.T) {
	condition := &ThresholdCondition{Count: 3, Window: 60}
	now := time.Now()
	ctx := &ConditionContext{
		State: &ConditionState{},
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
	if len(ctx.State.MatchTimes) != 3 {
		t.Fatalf("expected match times length 3, got %d", len(ctx.State.MatchTimes))
	}
}

func TestThresholdConditionEvalPrunesOutsideWindow(t *testing.T) {
	condition := &ThresholdCondition{Count: 2, Window: 10}
	now := time.Now()
	ctx := &ConditionContext{
		State: &ConditionState{},
	}

	ctx.Timestamp = now
	if got := condition.Eval(ctx); got != false {
		t.Fatalf("first event: expected false, got %v", got)
	}

	ctx.Timestamp = now.Add(15 * time.Second)
	if got := condition.Eval(ctx); got != false {
		t.Fatalf("second event after window: expected false, got %v", got)
	}
	if len(ctx.State.MatchTimes) != 1 {
		t.Fatalf("expected pruned match times length 1, got %d", len(ctx.State.MatchTimes))
	}

	ctx.Timestamp = now.Add(16 * time.Second)
	if got := condition.Eval(ctx); got != true {
		t.Fatalf("third event within window: expected true, got %v", got)
	}
	if len(ctx.State.MatchTimes) != 2 {
		t.Fatalf("expected final match times length 2, got %d", len(ctx.State.MatchTimes))
	}
}
