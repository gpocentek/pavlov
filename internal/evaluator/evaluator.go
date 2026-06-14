package evaluator

import (
	"context"
	"log/slog"
	"regexp"
	"time"

	"pavlov/internal/action"
	"pavlov/internal/condition"
	"pavlov/internal/config"
)

type LineEvent struct {
	Line      string
	Timestamp time.Time
}

type runState struct {
	LastFired time.Time
	Cancel    context.CancelFunc
	RunCtx    context.Context
}

// instanceState holds per-scope mutable state for a rule. The map key is the
// group_by capture value, or "" when group_by is not set. Condition tracks
// evaluation data (sliding windows, last seen); Run tracks cooldown and
// in-flight actions for stop_previous and timeout.
type instanceState struct {
	Condition *condition.ConditionState
	Run       runState
}

type Evaluator struct {
	Rule      *config.Rule
	Pattern   *regexp.Regexp
	Instances map[string]*instanceState
	events    chan LineEvent
}

func NewEvaluator(rule *config.Rule) *Evaluator {
	instances := make(map[string]*instanceState)
	if _, ok := rule.Condition.Value.(*condition.AbsenceCondition); ok && rule.GroupBy == "" {
		instances[""] = &instanceState{
			Condition: &condition.ConditionState{LastSeen: time.Now()},
		}
	}

	return &Evaluator{
		Rule:      rule,
		Pattern:   rule.PatternRegexp,
		Instances: instances,
		events:    make(chan LineEvent, 512),
	}
}

func (e *Evaluator) Run(ctx context.Context) {
	for {
		select {
		case event := <-e.events:
			e.process(event)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Evaluator) process(event LineEvent) bool {
	slog.Debug("processing line", "rule", e.Rule.Name, "line", event.Line)
	matches := e.Pattern.FindStringSubmatch(event.Line)
	if len(matches) == 0 {
		slog.Debug("pattern did not match", "rule", e.Rule.Name, "line", event.Line)
		return false
	}

	captures := make(map[string]string)
	for i, name := range e.Pattern.SubexpNames() {
		if name != "" && i < len(matches) {
			captures[name] = matches[i]
			slog.Debug("capture", "rule", e.Rule.Name, "name", name, "value", matches[i])
		}
	}

	scopeKey := ""
	if e.Rule.GroupBy != "" {
		scopeKey = captures[e.Rule.GroupBy]
	}

	state, ok := e.Instances[scopeKey]
	if !ok {
		state = &instanceState{Condition: &condition.ConditionState{}}
		e.Instances[scopeKey] = state
	}

	ctx := &condition.ConditionContext{
		Line:       event.Line,
		Captures:   captures,
		GroupValue: scopeKey,
		Timestamp:  event.Timestamp,
		State:      state.Condition,
	}

	if !e.Rule.Condition.Value.Eval(ctx) {
		return false
	}

	actionCtx := &action.ActionContext{
		Rule:      e.Rule.Name,
		File:      e.Rule.File,
		Line:      event.Line,
		GroupBy:   e.Rule.GroupBy,
		Group:     scopeKey,
		Timestamp: event.Timestamp,
		Captures:  captures,
	}
	return e.tryFire(state, actionCtx)
}

func (e *Evaluator) checkInstanceAbsence(scopeKey string, state *instanceState, timestamp time.Time) bool {
	ctx := &condition.ConditionContext{
		GroupValue: scopeKey,
		Timestamp:  timestamp,
		State:      state.Condition,
		FromTicker: true,
	}

	if !e.Rule.Condition.Value.Eval(ctx) {
		return false
	}

	actionCtx := &action.ActionContext{
		Rule:      e.Rule.Name,
		File:      e.Rule.File,
		GroupBy:   e.Rule.GroupBy,
		Group:     scopeKey,
		Timestamp: timestamp,
	}
	return e.tryFire(state, actionCtx)
}

func (e *Evaluator) CheckAbsence() {
	if _, ok := e.Rule.Condition.Value.(*condition.AbsenceCondition); !ok {
		return
	}

	now := time.Now()
	for scopeKey, state := range e.Instances {
		e.checkInstanceAbsence(scopeKey, state, now)
	}
}

func (e *Evaluator) tryFire(state *instanceState, actionCtx *action.ActionContext) bool {
	if active, expiresAt := e.inCooldown(state, actionCtx.Timestamp); active {
		slog.Info("Condition met, but cooldown not expired",
			"rule", e.Rule.Name,
			"group", actionCtx.Group,
			"cooldownExpiresAt", expiresAt,
		)
		return false
	}

	timeout := *e.Rule.Action.Value.RunOptions().Timeout
	stopPrevious := *e.Rule.Action.Value.RunOptions().StopPrevious

	slog.Info("Condition met, firing action", "rule", e.Rule.Name, "group", actionCtx.Group)
	state.Run.LastFired = actionCtx.Timestamp

	if stopPrevious && state.Run.RunCtx != nil && state.Run.RunCtx.Err() == nil {
		slog.Warn("Cancelling previous action", "rule", e.Rule.Name, "group", actionCtx.Group)
		state.Run.Cancel()
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(
			context.Background(),
			time.Duration(timeout)*time.Second,
		)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	go func() {
		if cancel != nil {
			defer cancel()
		}

		state.Run.Cancel = cancel
		state.Run.RunCtx = ctx
		slog.Debug("Starting action", "rule", e.Rule.Name, "group", actionCtx.Group)
		e.Rule.Action.Value.Act(ctx, actionCtx)
	}()
	return true
}

func (e *Evaluator) inCooldown(state *instanceState, timestamp time.Time) (bool, time.Time) {
	// Return (true, expiration time) if the cooldown is not expired, (false, time.Time{}) otherwise
	cooldown := time.Duration(e.Rule.Cooldown) * time.Second
	if state.Run.LastFired.IsZero() || timestamp.Sub(state.Run.LastFired) > cooldown {
		return false, time.Time{}
	}
	return true, state.Run.LastFired.Add(cooldown)
}

func (e *Evaluator) Enqueue(line string, timestamp time.Time) {
	select {
	case e.events <- LineEvent{Line: line, Timestamp: timestamp}:
	default:
		slog.Warn("event dropped, evaluator buffer full", "rule", e.Rule.Name)
	}
}
