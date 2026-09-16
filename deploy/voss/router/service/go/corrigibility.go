package main

import "sync"

type InterruptStore struct {
	mu        sync.Mutex
	interrupt map[string]bool
	corrected map[string]string
	terminate map[string]bool
}

func NewInterruptStore() *InterruptStore {
	return &InterruptStore{
		interrupt: make(map[string]bool),
		corrected: make(map[string]string),
		terminate: make(map[string]bool),
	}
}

func (s *InterruptStore) Interrupt(intentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interrupt[intentID] = true
}

func (s *InterruptStore) IsInterrupted(intentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interrupt[intentID]
}

func (s *InterruptStore) Clear(intentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.interrupt, intentID)
}

// Correct records an operator correction with the underlying rationale.
func (s *InterruptStore) Correct(intentID, directive string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.corrected[intentID] = directive
}

// Correction returns the operator correction directive and whether one exists.
func (s *InterruptStore) Correction(intentID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.corrected[intentID]
	return d, ok
}

func (s *InterruptStore) ClearCorrection(intentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.corrected, intentID)
}

// Terminate records a terminal operator termination for an intent.
func (s *InterruptStore) Terminate(intentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminate[intentID] = true
}

func (s *InterruptStore) IsTerminated(intentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminate[intentID]
}