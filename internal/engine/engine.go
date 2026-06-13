package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"pavlov/internal/condition"
	"pavlov/internal/config"
	"pavlov/internal/evaluator"
	"pavlov/internal/tailer"
)

type tailerManager struct {
	Tailer     *tailer.Tailer
	Evaluators []*evaluator.Evaluator
}

type Engine struct {
	cfg               *config.Config
	managers          map[string]*tailerManager
	absenceEvaluators []*evaluator.Evaluator
}

func NewEngine(cfg *config.Config) (*Engine, error) {
	managers := make(map[string]*tailerManager)
	absenceEvaluators := make([]*evaluator.Evaluator, 0)

	for _, rule := range cfg.Rules {
		mgr, ok := managers[rule.File]
		if !ok {
			tailer, err := tailer.NewTailer(rule.File)
			if err != nil {
				return nil, fmt.Errorf("Failed to create tailer: %v", err)
			}
			evaluators := make([]*evaluator.Evaluator, 0)
			mgr = &tailerManager{Tailer: tailer, Evaluators: evaluators}
			managers[rule.File] = mgr
		}

		ev := evaluator.NewEvaluator(rule)
		mgr.Evaluators = append(mgr.Evaluators, ev)
		if _, ok := rule.Condition.Value.(*condition.AbsenceCondition); ok {
			absenceEvaluators = append(absenceEvaluators, ev)
		}
	}

	engine := &Engine{
		cfg:               cfg,
		managers:          managers,
		absenceEvaluators: absenceEvaluators,
	}

	return engine, nil
}

func (e *Engine) Run(ctx context.Context) {
	ruleCount := 0
	for _, mgr := range e.managers {
		ruleCount += len(mgr.Evaluators)
	}
	slog.Info(
		"engine started",
		"files", len(e.managers),
		"rules", ruleCount,
		"absence_rules", len(e.absenceEvaluators),
	)

	for file, mgr := range e.managers {
		slog.Debug("starting tailer", "file", file, "rules", len(mgr.Evaluators))
		go func(file string, mgr *tailerManager) {
			if err := mgr.Tailer.Run(); err != nil {
				slog.Error("tailer stopped", "file", file, "err", err)
			}
		}(file, mgr)
		for _, ev := range mgr.Evaluators {
			go ev.Run(ctx)
		}
		go e.fanOut(mgr)
	}

	// Absence conditions need special handling, so we run a separate ticker for
	// them.
	if len(e.absenceEvaluators) > 0 {
		go e.runAbsenceTicker(ctx)
	}

	<-ctx.Done()
	slog.Info("engine stopped")
}

func (e *Engine) fanOut(mgr *tailerManager) {
	for line := range mgr.Tailer.Events {
		slog.Debug("line received", "file", mgr.Tailer.File, "line", line)
		now := time.Now()
		for _, ev := range mgr.Evaluators {
			ev.Enqueue(line, now)
		}
	}
}

func (e *Engine) runAbsenceTicker(ctx context.Context) {
	// TODO: Make this configurable.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, ev := range e.absenceEvaluators {
				ev.CheckAbsence()
			}
		case <-ctx.Done():
			return
		}
	}
}
