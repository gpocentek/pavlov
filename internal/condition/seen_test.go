package condition

import (
	"testing"
	"time"
)

func TestSeenCondition(t *testing.T) {
	condition := &SeenCondition{}
	err := condition.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if condition.String() != "seen()" {
		t.Fatalf("expected 'seen()', got %s", condition.String())
	}
}

func TestSeenConditionEval(t *testing.T) {
	now := time.Now()
	ctx := &ConditionContext{
		Timestamp: now,
		State:     &ConditionState{},
	}

	if got := (&SeenCondition{}).Eval(ctx); got != true {
		t.Fatalf("expected true, got %v", got)
	}
}
