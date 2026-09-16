package main

import "sync"

type InterruptStore struct {
	mu        sync.Mutex
	interrupt map[string]bool
}

func NewInterruptStore() *InterruptStore {
	return &InterruptStore{interrupt: make(map[string]bool)}
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