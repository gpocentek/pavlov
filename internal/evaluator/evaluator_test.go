package evaluator

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"pavlov/internal/action"
	"pavlov/internal/condition"
	"pavlov/internal/config"
)

const testPatternWithBackend = `error: (?P<backend>[a-z]+)`

// recordingAction is a test action that records the action context.
// When Act receives a context with a deadline, it blocks until cancelled
// and optionally reports ctx.Err() on finishedCh. This is useful for testing
// timeout handling in the evaluator.
type recordingAction struct {
	Options    action.RunOptions
	ch         chan *action.ActionContext
	finishedCh chan error
}

func newRecordingAction() *recordingAction {
	timeout := uint(0)
	stopPrevious := false
	return &recordingAction{
		Options: action.RunOptions{
			Timeout:      &timeout,
			StopPrevious: &stopPrevious,
		},
		ch: make(chan *action.ActionContext, 4),
	}
}

func (a *recordingAction) RunOptions() action.RunOptions {
	return a.Options
}

func (a *recordingAction) Act(ctx context.Context, actionCtx *action.ActionContext) {
	a.ch <- actionCtx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		return
	}
	<-ctx.Done()
	if a.finishedCh != nil {
		a.finishedCh <- ctx.Err()
	}
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
		Condition: config.NewConditionSpec(&condition.MatchCondition{}),
		Action:    config.ActionSpec{Value: newRecordingAction()},
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
	t.Run("ungrouped", func(t *testing.T) {
		rule := newTestRule(t, func(r *config.Rule) {
			r.Pattern = "heartbeat ok"
			r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
		})

		ev := NewEvaluator(rule)
		if len(ev.Instances) != 1 {
			t.Fatalf("expected one seeded scope, got %d", len(ev.Instances))
		}
		state, ok := ev.Instances[""]
		if !ok {
			t.Fatal(`expected "" scope to be seeded`)
		}
		if state.Condition.LastSeen.IsZero() {
			t.Fatal("expected LastSeen to be set at startup")
		}
	})

	t.Run("grouped", func(t *testing.T) {
		rule := newTestRule(t, func(r *config.Rule) {
			r.Pattern = "heartbeat ok"
			r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
			r.GroupBy = "service"
		})

		ev := NewEvaluator(rule)
		if len(ev.Instances) != 0 {
			t.Fatalf("expected no seeded states when group_by is set, got %d", len(ev.Instances))
		}
	})
}

func TestCheckPeriodicFiresWithoutMatchingLine(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()
	ev.Instances[""].Condition.LastSeen = now.Add(-15 * time.Second)

	ev.CheckPeriodic(context.Background(), now)
	waitForRecordedAction(t, recorder.ch)
}

func TestProcessNoMatch(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	now := time.Now()

	if got := ev.process(context.Background(), LineEvent{Line: "nothing here", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestProcessSeenFires(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	now := time.Now()
	line := "error: api"

	if got := ev.process(context.Background(), LineEvent{Line: line, Timestamp: now}); !got {
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
	if ev.Instances[""].Run.LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, ev.Instances[""].Run.LastFired)
	}
}

func TestProcessSeenCooldownBlocks(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 60
	}))
	now := time.Now()
	ev.Instances[""] = &instanceState{Run: runState{LastFired: now.Add(-10 * time.Second)}}

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if ev.Instances[""].Run.LastFired != now.Add(-10*time.Second) {
		t.Fatal("expected LastFired unchanged during cooldown")
	}
}

func TestProcessSeenCooldownExpired(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 60
	}))
	now := time.Now()
	ev.Instances[""] = &instanceState{Run: runState{LastFired: now.Add(-61 * time.Second)}}

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)
	if ev.Instances[""].Run.LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, ev.Instances[""].Run.LastFired)
	}
}

func TestProcessGroupBySeparateState(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.GroupBy = "backend"
		r.Cooldown = 60
	}))
	now := time.Now()

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("api: expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now.Add(time.Second)}); got {
		t.Fatalf("api cooldown: expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)

	if got := ev.process(context.Background(), LineEvent{Line: "error: db", Timestamp: now.Add(2 * time.Second)}); !got {
		t.Fatalf("db: expected true, got %v", got)
	}
	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Group != "db" {
		t.Fatalf("expected group db, got %q", actionCtx.Group)
	}
	if actionCtx.Captures["backend"] != "db" {
		t.Fatalf("expected backend=db, got %q", actionCtx.Captures["backend"])
	}
}

func TestProcessThresholdNotMetThenMet(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Condition = config.ConditionSpec{
			Value: &condition.ThresholdCondition{Count: 2, Window: 60},
		}
	}))
	now := time.Now()

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); got {
		t.Fatalf("first event: expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if len(ev.Instances[""].Condition.MatchTimes) != 1 {
		t.Fatalf("expected match times length 1, got %d", len(ev.Instances[""].Condition.MatchTimes))
	}

	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now.Add(time.Second)}); !got {
		t.Fatalf("second event: expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)
}

func TestProcessAbsenceUpdatesLastSeenNoFire(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()

	if got := ev.process(context.Background(), LineEvent{Line: "heartbeat ok", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if ev.Instances[""].Condition.LastSeen != now {
		t.Fatalf("expected LastSeen %v, got %v", now, ev.Instances[""].Condition.LastSeen)
	}
}

func TestProcessAbsenceGroupByCreatesPerGroupState(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = `heartbeat ok (?P<service>[a-z]+)`
		r.GroupBy = "service"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()

	if got := ev.process(context.Background(), LineEvent{Line: "heartbeat ok api", Timestamp: now}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
	if _, ok := ev.Instances["api"]; !ok {
		t.Fatal("expected api state to be created")
	}
	if ev.Instances["api"].Condition.LastSeen != now {
		t.Fatalf("expected LastSeen %v, got %v", now, ev.Instances["api"].Condition.LastSeen)
	}
	if _, ok := ev.Instances[""]; ok {
		t.Fatal(`did not expect "" state when group_by is set`)
	}
}

func TestCheckInstanceAbsenceNotMet(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()
	state := &instanceState{Condition: &condition.ConditionState{LastSeen: now.Add(-5 * time.Second)}}

	if got := ev.checkPeriodicInstance(context.Background(), "", state, now); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckInstanceAbsenceFires(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()
	state := &instanceState{Condition: &condition.ConditionState{LastSeen: now.Add(-15 * time.Second)}}

	if got := ev.checkPeriodicInstance(context.Background(), "", state, now); !got {
		t.Fatalf("expected true, got %v", got)
	}

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "" {
		t.Fatalf("expected empty line for absence action, got %q", actionCtx.Line)
	}
	if actionCtx.Group != "" {
		t.Fatalf("expected empty group, got %q", actionCtx.Group)
	}
	if state.Run.LastFired != now {
		t.Fatalf("expected LastFired %v, got %v", now, state.Run.LastFired)
	}
}

func TestCheckInstanceAbsenceCooldownBlocks(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = "heartbeat ok"
		r.Cooldown = 60
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()
	state := &instanceState{
		Condition: &condition.ConditionState{LastSeen: now.Add(-15 * time.Second)},
		Run:       runState{LastFired: now.Add(-10 * time.Second)},
	}

	if got := ev.checkPeriodicInstance(context.Background(), "", state, now); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckAbsenceSkipsNonAbsenceRule(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))
	ev.Instances[""] = &instanceState{Condition: &condition.ConditionState{LastSeen: time.Now().Add(-15 * time.Second)}}

	ev.CheckPeriodic(context.Background(), time.Now())
	assertNoRecordedAction(t, recorder.ch)
}

func TestCheckAbsenceChecksAllGroups(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Pattern = `heartbeat ok (?P<service>[a-z]+)`
		r.GroupBy = "service"
		r.Condition = config.NewConditionSpec(&condition.AbsenceCondition{Duration: 10})
	}))
	now := time.Now()
	ev.Instances["api"] = &instanceState{Condition: &condition.ConditionState{LastSeen: now.Add(-15 * time.Second)}}
	ev.Instances["worker"] = &instanceState{Condition: &condition.ConditionState{LastSeen: now.Add(-5 * time.Second)}}

	ev.CheckPeriodic(context.Background(), now)

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
	done := make(chan struct{})
	go func() {
		ev.Run(ctx)
		close(done)
	}()

	ev.Enqueue("error: api", now)
	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "error: api" {
		t.Fatalf("expected line error: api, got %q", actionCtx.Line)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestProcessSkipsWhenContextCanceled(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got := ev.process(ctx, LineEvent{Line: "error: api", Timestamp: time.Now()}); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestTryFireSkipsWhenContextCanceled(t *testing.T) {
	ev, recorder := newTestEvaluator(t, newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 0
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := &instanceState{Condition: &condition.ConditionState{}}
	actionCtx := &action.ActionContext{
		Rule:      ev.Rule.Name,
		File:      ev.Rule.File,
		Line:      "error: api",
		Timestamp: time.Now(),
	}

	if got := ev.tryFire(ctx, state, actionCtx); got {
		t.Fatalf("expected false, got %v", got)
	}
	assertNoRecordedAction(t, recorder.ch)
}

func TestRunShutdownWaitsForInFlightAction(t *testing.T) {
	recorder := newRecordingAction()
	*recorder.Options.Timeout = 60
	recorder.finishedCh = make(chan error, 1)

	ev := NewEvaluator(newTestRule(t, func(r *config.Rule) {
		r.Action = config.ActionSpec{Value: recorder}
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ev.Run(ctx)
		close(done)
	}()

	ev.Enqueue("error: api", time.Now())
	waitForRecordedAction(t, recorder.ch)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after shutdown")
	}

	select {
	case err := <-recorder.finishedCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected action canceled on shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected in-flight action to be canceled on shutdown")
	}
}

func TestEnqueueDropsWhenBufferFull(t *testing.T) {
	ev, _ := newTestEvaluator(t, newTestRule(t, nil))
	ev.events = make(chan LineEvent, 512)
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

func TestTimeoutKillsRunningAction(t *testing.T) {
	recorder := newRecordingAction()
	*recorder.Options.Timeout = 1
	recorder.finishedCh = make(chan error, 1)

	ev := NewEvaluator(newTestRule(t, func(r *config.Rule) {
		r.Action = config.ActionSpec{Value: recorder}
	}))

	now := time.Now()
	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "error: api" {
		t.Fatalf("expected line error: api, got %q", actionCtx.Line)
	}

	select {
	case err := <-recorder.finishedCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected action to be cancelled by timeout")
	}
}

func TestStopPreviousCancelsAndStartsNewAction(t *testing.T) {
	recorder := newRecordingAction()
	*recorder.Options.Timeout = 60
	*recorder.Options.StopPrevious = true
	recorder.finishedCh = make(chan error, 1)

	ev := NewEvaluator(newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 0
		r.Action = config.ActionSpec{Value: recorder}
	}))

	now := time.Now()
	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)

	if got := ev.process(context.Background(), LineEvent{Line: "error: db", Timestamp: now.Add(time.Second)}); !got {
		t.Fatalf("expected true, got %v", got)
	}

	select {
	case err := <-recorder.finishedCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected previous action cancelled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected previous action to be cancelled")
	}

	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "error: db" {
		t.Fatalf("expected line error: db, got %q", actionCtx.Line)
	}
}

func TestStopPreviousSkipsCompletedAction(t *testing.T) {
	recorder := newRecordingAction()
	*recorder.Options.StopPrevious = true
	recorder.finishedCh = make(chan error, 1)

	ev := NewEvaluator(newTestRule(t, func(r *config.Rule) {
		r.Cooldown = 0
		r.Action = config.ActionSpec{Value: recorder}
	}))

	now := time.Now()
	if got := ev.process(context.Background(), LineEvent{Line: "error: api", Timestamp: now}); !got {
		t.Fatalf("expected true, got %v", got)
	}
	waitForRecordedAction(t, recorder.ch)

	if got := ev.process(context.Background(), LineEvent{Line: "error: db", Timestamp: now.Add(time.Second)}); !got {
		t.Fatalf("expected true, got %v", got)
	}

	select {
	case err := <-recorder.finishedCh:
		t.Fatalf("unexpected cancellation of completed action: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	actionCtx := waitForRecordedAction(t, recorder.ch)
	if actionCtx.Line != "error: db" {
		t.Fatalf("expected line error: db, got %q", actionCtx.Line)
	}
}
