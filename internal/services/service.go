package services

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
)

// ErrAlreadyRunning is returned when you try to perform an invalid action
// while the service is running.
var ErrAlreadyRunning = errors.New("service is already running")

// Service describes the basic functionalities that all the services have in
// common.
type Service interface {
	Run() error
	Close(ctx context.Context)
}

// service is the base struct for the services; it manages the running state,
// logger, and the waiting group of processes.
type service struct {
	running bool
	logger  *zap.SugaredLogger
	wg      sync.WaitGroup
}

// Run makes all the tasks related to the starting process for any service,
// like updating the running state and writing the logs.
func (s *service) Run() error {
	// Check if the service is already started
	if s.Running() {
		// notest
		return ErrAlreadyRunning
	}
	s.running = true
	s.logger.Info("Service started")
	return nil
}

// Close makes all the tasks related to the close process for any service, like
// updating the running state, writing the logs, and waiting for the active
// process.
func (s *service) Close(_ context.Context) {
	// Check if the service is already running
	if !s.Running() {
		// notest
		s.logger.Warn("service is not running")
		return
	}

	s.running = false

	s.logger.Info("Waiting for finish the active process")
	s.wg.Wait()
	s.logger.Info("Service stopped")
}

// AddProcess must be used at the beginning of each service process. It adds 1
// to the counter of the waiting group of process.
func (s *service) AddProcess() {
	if !s.Running() {
		// notest
		return
	}
	s.wg.Add(1)
}

// DoneProcess must be used at the end of each service process. It decreases by
// 1 the counter of the waiting group of processes.
func (s *service) DoneProcess() {
	s.wg.Done()
}

// Running returns true if the service is running, and false in another case.
func (s *service) Running() bool {
	return s.running
}
