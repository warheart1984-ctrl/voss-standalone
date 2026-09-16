package main

import "sync"

type CorrigibilityAPI struct {
	mu sync.Mutex
	interrupts map[string]bool
}

func NewCorrigibilityAPI() *CorrigibilityAPI {
	return &CorrigibilityAPI{interrupts: make(map[string]bool)}
}

func (c *CorrigibilityAPI) Interrupt(intentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interrupts[intentID] = true
}

func (c *CorrigibilityAPI) IsInterrupted(intentID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.interrupts[intentID]
}

func (c *CorrigibilityAPI) Clear(intentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.interrupts, intentID)
}
