package condition

import (
	"testing"
	"time"
)

func TestMatchCondition(t *testing.T) {
	condition := &MatchCondition{}
	err := condition.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if condition.String() != "match()" {
		t.Fatalf("expected 'match()', got %s", condition.String())
	}
}

func TestMatchConditionEval(t *testing.T) {
	now := time.Now()
	ctx := &ConditionContext{
		Timestamp: now,
		State:     &ConditionState{},
	}

	if got := (&MatchCondition{}).Eval(ctx); got != true {
		t.Fatalf("expected true, got %v", got)
	}
}
