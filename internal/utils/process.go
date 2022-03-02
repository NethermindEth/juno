package utils

import (
	"context"
	"github.com/jesselucas/shutdown"
	"time"

	"go.uber.org/zap"
)

// runFunction represent the structure of the function that Run a process
type runFunction func(logger *zap.SugaredLogger) error

// stopFunction represent the structure of the function that Stop a process
type stopFunction func(ctx context.Context)

// process contains information and required functions of each process
type process struct {
	id     string
	err    error
	logger *zap.SugaredLogger
	run    runFunction
	stop   stopFunction
}

// Start the process
func (proc *process) Start() (err error) {
	return proc.run(proc.logger)
}

type Processor interface {
	Add(logger *zap.SugaredLogger, id string, fnRun runFunction, fnStop stopFunction)
	Run()
	Close()
}

// ProcessorRunner collects sub processes
type ProcessorRunner struct {
	list     []*process
	Shutdown *shutdown.Shutdown
}

// NewProcessor creates a new ProcessorRunner
func NewProcessor() *ProcessorRunner {
	return &ProcessorRunner{Shutdown: shutdown.NewShutdown()}
}

// Close all processes
func (p *ProcessorRunner) Close() {
	// Clear all process
	for _, proc := range p.list {
		// Set 5 seconds as timeout
		ctx, cancelFunc := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		proc.stop(ctx)
		if proc.err != nil {
			proc.logger.With("Error", proc.err).Error("Error in execution")
		}
		cancelFunc()
	}
}

// Add a process
func (p *ProcessorRunner) Add(logger *zap.SugaredLogger, id string, fnRun runFunction, fnStop stopFunction) {
	p.list = append(p.list, &process{
		id:     id,
		logger: logger.With(zap.String("process", id)),
		run:    fnRun,
		stop:   fnStop,
	})
}

// Run all processes
func (p *ProcessorRunner) Run() {
	// go through all registered processes
	for _, proc := range p.list {
		// start process
		go func(process *process) {
			err := process.Start()
			if err != nil {
				process.err = err
				return
			}
		}(proc)
	}

	// Create an infinite loop
	select {}
}
