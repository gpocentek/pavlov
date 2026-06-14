package evaluator

import (
	"context"
	"regexp"
	"testing"
	"time"

	"pavlov/internal/action"
	"pavlov/internal/condition"
	"pavlov/internal/config"
)

const testPatternWithBackend = `error: (?P<backend>[a-z]+)`

// recordingAction is a test action that records the action context
type recordingAction struct {
	ch chan *action.ActionContext
}

func newRecordingAction() *recordingAction {
	return &recordingAction{ch: make(chan *action.ActionContext, 4)}
}

func (a *recordingAction) Act(ctx *action.ActionContext) {
	a.ch <- ctx
}

func (a *recordingAction) Validate() error {
	return nil
}

func newTestRule(t *testing.T, opts func(*config.Rule)) *config.Rule {
	t.Helper()
	rule := &config.Rule{
		Name:      "test_rule",
		File:      "/tmp/test.log",
		Pattern:   testPatternWithBackend,
		Condition: config.ConditionConfig{Value: &condition.SeenCondition{}},
		Action:    config.ActionConfig{Value: newRecordingAction()},
	}
	if opts != nil {
		opts(rule)
	}
	rule.PatternRegexp = regexp.MustCompile(rule.Pattern)
	return rule
}

func newTestEvaluator(t *testing.T, rule *config.Rule) (*Evaluator, *recordingAction) {
	t.Helper()
	recorder, ok := rule.Action.Value.(*recordingAction)
	if !ok {
		t.Fatal("expected recordingAction in test rule")
	}
	return NewEvaluator(rule), recorder
}

func waitForRecordedAction(t *testing.T, ch <-chan *action.ActionContext) *action.ActionContext {
	t.Helper()
	select {
	case actionCtx := <-ch:
		return actionCtx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for action")
		return nil
	}
}

func assertNoRecordedAction(t *testing.T, ch <-chan *action.ActionContext) {
	t.Helper()
	select {
	case actionCtx := <-ch:
		t.Fatalf("unexpected action: %+v", actionCtx)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNewEvaluatorAbsenceSeed(t *testing.T) {
	rule := newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	})

	ev := NewEvaluator(rule)
	if _, ok := ev.States[""]; !ok {
		t.Fatal(`expected "" state to be seeded when group_by is empty`)
	}
	if len(ev.States) != 1 {
		t.Fatalf("expected 1 state, got %d", len(ev.States))
	}

	rule.GroupBy = "service"
	ev = NewEvaluator(rule)
	if len(ev.States) != 0 {
		t.Fatalf("expected no seeded states when group_by is set, got %d", len(ev.States))
	}
}

func TestProcessNoMatch(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	now := time.Now()

	if got := ev.process(Event{Line: "nothing here", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestProcessSeenFires(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	now := time.Now()
	line := "error: api"

	if got := ev.process(Event{Line: line, Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Rule != "test_rule" {
		t.Fatalf("expected rule test_rule, got %q", actionCtx.Rule)
	}
	if actionCtx.Line != line {
		t.Fatalf("expected line %q, got %q", line, actionCtx.Line)
	}
	if actionCtx.Group != "" {
		t.Fatalf("expected empty group, got %q", actionCtx.Group)
	}
	if ev.States[""].LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, ev.States[""].LastFired)
	}
}

func TestProcessSeenCooldownBlocks(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 60
	}))
	now := time.Now()
	ev.States[""] = &condition.GroupState{LastFired: now.Add(-10 * time.Second)}

	if got := ev.process(Event{Line: "error: api", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if ev.States[""].LastFired != now.Add(-10*time.Second) {
		t.Fatal("expected LastFired unchanged during cooldown")
	}
}

func TestProcessSeenCooldownExpired(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 60
	}))
	now := time.Now()
	ev.States[""] = &condition.GroupState{LastFired: now.Add(-61 * time.Second)}

	if got := ev.process(Event{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)
	if ev.States[""].LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, ev.States[""].LastFired)
	}
}

func TestProcessGroupBySeparateState(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.GroupBy = "backend"
		r.Cooldown = 60
	}))
	now := time.Now()

	if got := ev.process(Event{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("api: expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)

	if got := ev.process(Event{Line: "error: api", Timestamp: now.Add(time.Second)}); got {
		t.Fatalf("api cooldown: expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)

	if got := ev.process(Event{Line: "error: db", Timestamp: now.Add(2 * time.Second)}); !got {
		t.Fatalf("db: expected true, got %v", got)
	}
	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Group != "db" {
		t.Fatalf("expected group db, got %q", actionCtx.Group)
	}
	if actionCtx.Vars["backend"] != "db" {
		t.Fatalf("expected backend=db, got %q", actionCtx.Vars["backend"])
	}
}

func TestProcessThresholdNotMetThenMet(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Condition = config.ConditionConfig{
			Value: &condition.ThresholdCondition{Threshold: 2, Window: 60},
		}
	}))
	now := time.Now()

	if got := ev.process(Event{Line: "error: api", Timestamp: now}); got {
		t.Fatalf("first event: expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if len(ev.States[""].Window) != 1 {
		t.Fatalf("expected window length 1, got %d", len(ev.States[""].Window))
	}

	if got := ev.process(Event{Line: "error: api", Timestamp: now.Add(time.Second)}); !got {
		t.Fatalf("second event: expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)
}

func TestProcessAbsenceUpdatesLastSeenNoFire(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()

	if got := ev.process(Event{Line: "heartbeat ok", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if ev.States[""].LastSeen != now {
		t.Fatalf("expected LastSeen %v, got %v", now, ev.States[""].LastSeen)
	}
}

func TestProcessAbsenceGroupByCreatesPerGroupState(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = `heartbeat ok (?P<service>[a-z]+)`
		r.GroupBy = "service"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()

	if got := ev.process(Event{Line: "heartbeat ok api", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if _, ok := ev.States["api"]; !ok {
		t.Fatal("expected api state to be created")
	}
	if ev.States["api"].LastSeen != now {
		t.Fatalf("expected LastSeen %v, got %v", now, ev.States["api"].LastSeen)
	}
	if _, ok := ev.States[""]; ok {
		t.Fatal(`did not expect "" state when group_by is set`)
	}
}

func TestCheckGroupAbsenceNotMet(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()
	state := &condition.GroupState{LastSeen: now.Add(-5 * time.Second)}

	if got := ev.checkGroupAbsence("", state, now); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckGroupAbsenceFires(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()
	state := &condition.GroupState{LastSeen: now.Add(-15 * time.Second)}

	if got := ev.checkGroupAbsence("", state, now); !got {
		t.Fatalf("expected true, got %v", got)
	}

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "" {
		t.Fatalf("expected empty line for absence action, got %q", actionCtx.Line)
	}
	if actionCtx.Group != "" {
		t.Fatalf("expected empty group, got %q", actionCtx.Group)
	}
	if state.LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, state.LastFired)
	}
}

func TestCheckGroupAbsenceCooldownBlocks(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Cooldown = 60
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()
	state := &condition.GroupState{
		LastSeen:  now.Add(-15 * time.Second),
		LastFired: now.Add(-10 * time.Second),
	}

	if got := ev.checkGroupAbsence("", state, now); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckAbsenceSkipsNonAbsenceRule(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	ev.States[""] = &condition.GroupState{LastSeen: time.Now().Add(-15 * time.Second)}

	ev.CheckAbsence()
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckAbsenceChecksAllGroups(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = `heartbeat ok (?P<service>[a-z]+)`
		r.GroupBy = "service"
		r.Condition = config.ConditionConfig{Value: &condition.AbsenceCondition{Duration: 10}}
	}))
	now := time.Now()
	ev.States["api"] = &condition.GroupState{LastSeen: now.Add(-15 * time.Second)}
	ev.States["worker"] = &condition.GroupState{LastSeen: now.Add(-5 * time.Second)}

	ev.CheckAbsence()

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Group != "api" {
		t.Fatalf("expected api to fire, got group %q", actionCtx.Group)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestEnqueueEnqueuesEvent(t *testing.T) {
	ev, _ := newTestEvaluator(t, newTestRule(t, nil))
	now := time.Now()

	ev.Enqueue("error: api", now)

	select {
	case event := <-ev.events:
		if event.Line != "error: api" {
			t.Fatalf("expected line error: api, got %q", event.Line)
		}
		if event.Timestamp != now {
			t.Fatalf("expected timestamp %v, got %v", now, event.Timestamp)
		}
	case <-time.After(time.Second):
		t.Fatal("expected event on channel")
	}
}

func TestEnqueueRunProcessesEvent(t *testing.T) {
	rule := newTestRule(t, nil)
	ev, recorder := newTestEvaluator(t, rule)
	now := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ev.Run(ctx)

	ev.Enqueue("error: api", now)
	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "error: api" {
		t.Fatalf("expected line error: api, got %q", actionCtx.Line)
	}

	cancel()
}

func TestEnqueueDropsWhenBufferFull(t *testing.T) {
	ev, _ := newTestEvaluator(t, newTestRule(t, nil))
	ev.events = make(chan Event, 512)
	for range 512 {
		ev.Enqueue("error: api", time.Now())
	}

	done := make(chan struct{})
	go func() {
		ev.Enqueue("error: db", time.Now())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue blocked on full buffer")
	}
}
