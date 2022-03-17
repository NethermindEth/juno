// Package process provides a handler to manage processes.
package process

// notest
import (
	"context"
	"time"

	"github.com/NethermindEth/juno/internal/log"
)

// runFunc represents a function that runs a process.
type runFunc func() error

// stopFunc represents a function that stops a process.
type stopFunc func(ctx context.Context)

// process contains information and required functions of each process
type process struct {
	id   string
	err  error
	run  runFunc
	stop stopFunc
}

// Processer defines common methods for a context processor.
type Processer interface {
	Add(id string, fnRun runFunc, fnStop stopFunc)
	Run()
	Close()
}

// Handler holds a collection of sub-processes.
type Handler struct {
	subprocs []*process
}

// NewHandler creates a new process.Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Start starts a process.
func (p *process) Start() (err error) {
	return p.run()
}

// Close closes all running processes.
func (h *Handler) Close() {
	// Clear all processes.
	for _, proc := range h.subprocs {
		// Set 5 second timeout.
		ctx, cancel := context.WithDeadline(
			context.Background(), time.Now().Add(5*time.Second))
		proc.stop(ctx)
		if proc.err != nil {
			log.Default.With("Error", proc.err).Error("Error occurred while closing process.")
		}
		cancel()
	}
}

// Add adds a process to the list of processes in the Handler.
func (h *Handler) Add(id string, fnRun runFunc, fnStop stopFunc) {
	h.subprocs = append(h.subprocs, &process{
		id:   id,
		run:  fnRun,
		stop: fnStop,
	})
}

// Run runs all the processes.
func (h *Handler) Run() {
	if len(h.subprocs) == 0 {
		log.Default.Info("No processes found.")
		return
	}

	for _, proc := range h.subprocs {
		go func(p *process) {
			err := p.Start()
			if err != nil {
				p.err = err
				return
			}
		}(proc)
	}

	// Create an infinite loop.
	select {}
}
