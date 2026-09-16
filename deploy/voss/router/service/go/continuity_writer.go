package main

import (
	"encoding/json"
	"os"
	"sync"
)

type ContinuityWriter struct {
	path string
	mu   sync.Mutex
}

func NewContinuityWriter(path string) *ContinuityWriter {
	return &ContinuityWriter{path: path}
}

func (c *ContinuityWriter) Write(entry LedgerEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil { return err }
	defer f.Close()
	b, _ := json.Marshal(entry)
	f.WriteString(string(b))
	f.WriteString("\n")
	return nil
}
