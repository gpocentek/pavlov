package condition

import (
	"fmt"
	"testing"
	"time"
)

func TestAbsenceCondition(t *testing.T) {
	duration := 10
	condition := &AbsenceCondition{Duration: uint(duration)}
	err := condition.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	conditionStr := fmt.Sprintf("absence(duration=%d)", duration)
	if condition.String() != conditionStr {
		t.Fatalf("expected '%s', got %s", conditionStr, condition.String())
	}
}

func TestAbsenceConditionValidateFail(t *testing.T) {
	condition := &AbsenceCondition{Duration: 0}
	err := condition.Validate()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "`duration` must be defined and greater than 0" {
		t.Fatalf("expected '`duration` must be defined and greater than 0', got %s", err.Error())
	}
}

func TestAbsenceConditionEvalEvent(t *testing.T) {
	condition := &AbsenceCondition{Duration: 10}
	now := time.Now()
	ctx := &ConditionContext{
		Timestamp: now,
		State:     &ConditionState{},
	}

	if got := condition.Eval(ctx); got != false {
		t.Fatalf("expected false, got %v", got)
	}
	if ctx.State.LastSeen != now {
		t.Fatalf("expected last seen to be %v, got %v", now, ctx.State.LastSeen)
	}
}

func TestAbsenceConditionEvalPeriodic(t *testing.T) {
	condition := &AbsenceCondition{Duration: 10}
	now := time.Now()

	// 15 seconds after the last seen, the absence should be true
	ctx := &ConditionContext{
		Timestamp: now.Add(15 * time.Second),
		State:     &ConditionState{LastSeen: now},
	}

	if got := condition.EvalPeriodic(ctx); got != true {
		t.Fatalf("expected true, got %v", got)
	}

	// 5 seconds after the last seen, the absence should be false
	ctx = &ConditionContext{
		Timestamp: now.Add(5 * time.Second),
		State:     &ConditionState{LastSeen: now},
	}

	if got := condition.EvalPeriodic(ctx); got != false {
		t.Fatalf("expected false, got %v", got)
	}
}

func TestAbsenceConditionEvalPeriodicExactDuration(t *testing.T) {
	condition := &AbsenceCondition{Duration: 10}
	now := time.Now()

	ctx := &ConditionContext{
		Timestamp: now.Add(10 * time.Second),
		State:     &ConditionState{LastSeen: now},
	}
	if got := condition.EvalPeriodic(ctx); got != false {
		t.Fatalf("expected false at exact duration, got %v", got)
	}

	ctx.Timestamp = now.Add(10*time.Second + time.Nanosecond)
	if got := condition.EvalPeriodic(ctx); got != true {
		t.Fatalf("expected true just past duration, got %v", got)
	}
}

func TestAbsenceConditionSeedInstances(t *testing.T) {
	condition := &AbsenceCondition{Duration: 10}

	if seeds := condition.SeedInstances("service"); seeds != nil {
		t.Fatalf("expected nil seeds with group_by, got %v", seeds)
	}

	seeds := condition.SeedInstances("")
	if len(seeds) != 1 {
		t.Fatalf("expected one seed, got %d", len(seeds))
	}
	state, ok := seeds[""]
	if !ok || state.LastSeen.IsZero() {
		t.Fatal(`expected "" scope with LastSeen set`)
	}
}
