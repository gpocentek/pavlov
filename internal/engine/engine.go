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

type filePipeline struct {
	Tailer     *tailer.Tailer
	Evaluators []*evaluator.Evaluator
}

type Engine struct {
	cfg               *config.Config
	pipelines         map[string]*filePipeline
	absenceEvaluators []*evaluator.Evaluator
}

func NewEngine(cfg *config.Config) (*Engine, error) {
	pipelines := make(map[string]*filePipeline)
	absenceEvaluators := make([]*evaluator.Evaluator, 0)

	for _, rule := range cfg.Rules {
		pipeline, ok := pipelines[rule.File]
		if !ok {
			tailer, err := tailer.NewTailer(rule.File)
			if err != nil {
				return nil, fmt.Errorf("failed to create tailer: %v", err)
			}
			evaluators := make([]*evaluator.Evaluator, 0)
			pipeline = &filePipeline{Tailer: tailer, Evaluators: evaluators}
			pipelines[rule.File] = pipeline
		}

		ev := evaluator.NewEvaluator(rule)
		pipeline.Evaluators = append(pipeline.Evaluators, ev)
		if _, ok := rule.Condition.Value.(*condition.AbsenceCondition); ok {
			absenceEvaluators = append(absenceEvaluators, ev)
		}
	}

	engine := &Engine{
		cfg:               cfg,
		pipelines:         pipelines,
		absenceEvaluators: absenceEvaluators,
	}

	return engine, nil
}

func (e *Engine) Run(ctx context.Context) {
	ruleCount := 0
	for _, pipeline := range e.pipelines {
		ruleCount += len(pipeline.Evaluators)
	}
	slog.Info(
		"engine started",
		"files", len(e.pipelines),
		"rules", ruleCount,
		"absence_rules", len(e.absenceEvaluators),
	)

	for file, pipeline := range e.pipelines {
		slog.Debug("starting tailer", "file", file, "rules", len(pipeline.Evaluators))
		go func(file string, pipeline *filePipeline) {
			if err := pipeline.Tailer.Run(); err != nil {
				slog.Error("tailer stopped", "file", file, "err", err)
			}
		}(file, pipeline)
		for _, ev := range pipeline.Evaluators {
			go ev.Run(ctx)
		}
		go e.fanOut(pipeline)
	}

	// Absence conditions need special handling, so we run a separate ticker for
	// them.
	if len(e.absenceEvaluators) > 0 {
		go e.runAbsenceTicker(ctx)
	}

	<-ctx.Done()
	slog.Info("engine stopped")
}

func (e *Engine) fanOut(pipeline *filePipeline) {
	for line := range pipeline.Tailer.Events {
		slog.Debug("line received", "file", pipeline.Tailer.File, "line", line)
		now := time.Now()
		for _, ev := range pipeline.Evaluators {
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
