package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pavlov/internal/config"
	"pavlov/internal/evaluator"
	"pavlov/internal/tailer"
)

type filePipeline struct {
	Tailer     *tailer.Tailer
	Evaluators []*evaluator.Evaluator
}

type Engine struct {
	cfg       *config.Config
	pipelines map[string]*filePipeline
}

func NewEngine(cfg *config.Config) (*Engine, error) {
	pipelines := make(map[string]*filePipeline)

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
	}

	engine := &Engine{
		cfg:       cfg,
		pipelines: pipelines,
	}

	return engine, nil
}

func (e *Engine) Run(ctx context.Context) {
	wg := sync.WaitGroup{}
	ruleCount := 0
	for _, pipeline := range e.pipelines {
		ruleCount += len(pipeline.Evaluators)
	}
	slog.Info(
		"engine started",
		"files", len(e.pipelines),
		"rules", ruleCount,
	)

	for file, pipeline := range e.pipelines {
		slog.Debug("starting tailer", "file", file, "rules", len(pipeline.Evaluators))
		wg.Add(1)
		go func(file string, pipeline *filePipeline) {
			defer wg.Done()
			if err := pipeline.Tailer.Run(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("tailer stopped", "file", file, "err", err)
			}
		}(file, pipeline)
		for _, ev := range pipeline.Evaluators {
			wg.Add(1)
			go func(ev *evaluator.Evaluator) {
				defer wg.Done()
				ev.Run(ctx)
			}(ev)
		}
		wg.Add(1)
		go func(p *filePipeline) {
			defer wg.Done()
			e.fanOut(ctx, p)
		}(pipeline)
	}

	wg.Go(func() {
		e.runPeriodicTicker(ctx)
	})

	<-ctx.Done()
	wg.Wait()
	slog.Info("engine stopped")
}

func (e *Engine) fanOut(ctx context.Context, pipeline *filePipeline) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-pipeline.Tailer.Events:
			if !ok {
				return
			}
			slog.Debug("line received", "file", pipeline.Tailer.File, "line", line)
			now := time.Now()
			for _, ev := range pipeline.Evaluators {
				ev.Enqueue(line, now)
			}
		}
	}
}

func (e *Engine) runPeriodicTicker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for _, pipeline := range e.pipelines {
				for _, ev := range pipeline.Evaluators {
					ev.CheckPeriodic(ctx, now)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
