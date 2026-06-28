package engine

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"pavlov/internal/action"
	"pavlov/internal/condition"
	"pavlov/internal/config"
)

type blockingAction struct {
	started chan struct{}
	done    chan struct{}
	opts    action.RunOptions
}

func newBlockingAction() *blockingAction {
	timeout := uint(0)
	stopPrevious := false
	return &blockingAction{
		started: make(chan struct{}, 1),
		done:    make(chan struct{}),
		opts: action.RunOptions{
			Timeout:      &timeout,
			StopPrevious: &stopPrevious,
		},
	}
}

func (a *blockingAction) RunOptions() action.RunOptions {
	return a.opts
}

func (a *blockingAction) Validate() error {
	return nil
}

func (a *blockingAction) Act(ctx context.Context, _ *action.ActionContext) {
	a.started <- struct{}{}
	<-ctx.Done()
	close(a.done)
}

func TestEngineRunShutdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	if err := os.WriteFile(logFile, []byte("error: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logAction := &action.LogAction{Template: "{{ .Line }}"}
	if err := logAction.Validate(); err != nil {
		t.Fatal(err)
	}

	rule := &config.Rule{
		Name:      "test_rule",
		File:      logFile,
		Pattern:   `error: (?P<backend>[a-z]+)`,
		Condition: config.ConditionSpec{Value: &condition.MatchCondition{}},
		Action:    config.ActionSpec{Value: logAction},
	}
	rule.PatternRegexp = regexp.MustCompile(rule.Pattern)

	e, err := NewEngine(&config.Config{Rules: []*config.Rule{rule}})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not stop after context cancel")
	}
}

func TestEngineRunShutdownWaitsForInFlightAction(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	if err := os.WriteFile(logFile, []byte("error: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	blocker := newBlockingAction()
	rule := &config.Rule{
		Name:      "test_rule",
		File:      logFile,
		Pattern:   `error: (?P<backend>[a-z]+)`,
		Condition: config.ConditionSpec{Value: &condition.MatchCondition{}},
		Action:    config.ActionSpec{Value: blocker},
	}
	rule.PatternRegexp = regexp.MustCompile(rule.Pattern)

	e, err := NewEngine(&config.Config{Rules: []*config.Rule{rule}})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	engineDone := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(engineDone)
	}()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(logFile, []byte("error: api\nerror: api\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-blocker.started:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for action to start")
	}

	cancel()

	select {
	case <-engineDone:
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not stop while action was in flight")
	}

	select {
	case <-blocker.done:
	case <-time.After(3 * time.Second):
		t.Fatal("action did not finish after shutdown cancel")
	}
}
