package utils

import (
	"context"
	"github.com/NethermindEth/juno/internal/log"
	"time"
)

// runFunction represent the structure of the function that Run a process
type runFunction func() error

// stopFunction represent the structure of the function that Stop a process
type stopFunction func(ctx context.Context)

// process contains information and required functions of each process
type process struct {
	id   string
	err  error
	run  runFunction
	stop stopFunction
}

// Start the process
func (proc *process) Start() (err error) {
	return proc.run()
}

type Processor interface {
	Add(id string, fnRun runFunction, fnStop stopFunction)
	Run()
	Close()
}

// ProcessorRunner collects sub processes
type ProcessorRunner struct {
	list []*process
}

// NewProcessor creates a new ProcessorRunner
func NewProcessor() *ProcessorRunner {
	return &ProcessorRunner{}
}

// Close all processes
func (p *ProcessorRunner) Close() {
	// Clear all process
	for _, proc := range p.list {
		// Set 5 seconds as timeout
		ctx, cancelFunc := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		proc.stop(ctx)
		if proc.err != nil {
			log.Default.With("Error", proc.err).Error("Error in execution")
		}
		cancelFunc()
	}
}

// Add a process
func (p *ProcessorRunner) Add(id string, fnRun runFunction, fnStop stopFunction) {
	p.list = append(p.list, &process{
		id:   id,
		run:  fnRun,
		stop: fnStop,
	})
}

// Run all processes
func (p *ProcessorRunner) Run() {
	// If we don't have any process to run, just continue
	if len(p.list) == 0 {
		log.Default.Info("Not found any process to run in background")
		return
	}

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
