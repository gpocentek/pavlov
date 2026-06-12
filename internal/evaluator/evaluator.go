package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"pavlov/internal/action"
	"pavlov/internal/condition"
	"pavlov/internal/config"
)

type Event struct {
	Line      string
	Timestamp time.Time
}

type Evaluator struct {
	Rule    *config.Rule
	Pattern *regexp.Regexp
	States  map[string]*condition.GroupState
	events  chan Event
}

func NewEvaluator(rule *config.Rule) (*Evaluator, error) {
	compiledRe, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return nil, fmt.Errorf("Invalid regex '%s': %v", rule.Pattern, err)
	}

	states := make(map[string]*condition.GroupState)
	if _, ok := rule.Condition.Value.(*condition.AbsenceCondition); ok {
		states[""] = &condition.GroupState{LastSeen: time.Now()}
	}

	return &Evaluator{
		Rule:    rule,
		Pattern: compiledRe,
		States:  states,
		events:  make(chan Event, 512),
	}, err
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

func (e *Evaluator) process(event Event) {
	slog.Debug("processing line", "rule", e.Rule.Name, "line", event.Line)
	matches := e.Pattern.FindStringSubmatch(event.Line)
	if len(matches) == 0 {
		slog.Debug("pattern did not match", "rule", e.Rule.Name, "line", event.Line)
		return
	}

	vars := make(map[string]string)
	for i, name := range e.Pattern.SubexpNames() {
		if name != "" && i < len(matches) {
			vars[name] = matches[i]
			slog.Debug("capture", "rule", e.Rule.Name, "name", name, "value", matches[i])
		}
	}

	group := ""
	if e.Rule.GroupBy != "" {
		group = vars[e.Rule.GroupBy]
	}

	state, ok := e.States[group]
	if !ok {
		state = &condition.GroupState{}
		e.States[group] = state
	}

	ctx := &condition.ConditionContext{
		Line:      event.Line,
		Vars:      vars,
		Group:     group,
		Cooldown:  e.Rule.Cooldown,
		Timestamp: event.Timestamp,
		State:     state,
	}

	if !e.Rule.Condition.Value.Eval(ctx) {
		return
	}

	slog.Info("Condition met, firing action", "rule", e.Rule.Name, "group", group)
	actionCtx := &action.ActionContext{
		Rule:      e.Rule.Name,
		File:      e.Rule.File,
		Line:      event.Line,
		GroupBy:   e.Rule.GroupBy,
		Group:     group,
		Timestamp: event.Timestamp,
		Vars:      vars,
	}
	go e.Rule.Action.Value.Act(actionCtx)
}

func (e *Evaluator) CheckAbsence() {
	if _, ok := e.Rule.Condition.Value.(*condition.AbsenceCondition); !ok {
		return
	}

	now := time.Now()
	for group, state := range e.States {
		ctx := &condition.ConditionContext{
			Group:       group,
			Cooldown:    e.Rule.Cooldown,
			Timestamp:   now,
			State:       state,
			AbsenceTick: true,
		}

		if !e.Rule.Condition.Value.Eval(ctx) {
			continue
		}

		slog.Info("Absence detected, firing action", "rule", e.Rule.Name, "group", group)
		actionCtx := &action.ActionContext{
			Rule:      e.Rule.Name,
			File:      e.Rule.File,
			GroupBy:   e.Rule.GroupBy,
			Group:     group,
			Timestamp: now,
		}
		go e.Rule.Action.Value.Act(actionCtx)
	}
}

func (e *Evaluator) Enqueue(line string, timestamp time.Time) {
	select {
	case e.events <- Event{Line: line, Timestamp: timestamp}:
	default:
		slog.Warn("event dropped, evaluator buffer full", "rule", e.Rule.Name)
	}
}
