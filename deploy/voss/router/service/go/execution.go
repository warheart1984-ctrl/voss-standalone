package main

import "sync"

type ExecState string

const (
	ExecPending    ExecState = "PENDING"
	ExecAdmitted   ExecState = "ADMITTED"
	ExecRunning    ExecState = "RUNNING"
	ExecHalted     ExecState = "HALTED"
	ExecTerminated ExecState = "TERMINATED"
)

// legalTransitions defines the execution state machine. HALTED and TERMINATED
// are terminal: no outgoing transitions permitted.
var legalTransitions = map[ExecState]map[ExecState]bool{
	ExecPending:  {ExecAdmitted: true},
	ExecAdmitted: {ExecRunning: true, ExecHalted: true, ExecTerminated: true},
	ExecRunning:  {ExecHalted: true, ExecTerminated: true},
	ExecHalted:   {},
	ExecTerminated: {},
}

func (s ExecState) IsTerminal() bool {
	return s == ExecHalted || s == ExecTerminated
}

type ExecutionState struct {
	mu     sync.Mutex
	states map[string]ExecState
}

func NewExecutionState() *ExecutionState {
	return &ExecutionState{states: make(map[string]ExecState)}
}

func (e *ExecutionState) Current(intentID string) ExecState {
	e.mu.Lock()
	defer e.mu.Unlock()
	st, ok := e.states[intentID]
	if !ok {
		return ExecPending
	}
	return st
}

// Transition advances from expect to next only if legal. Returns false
// (including silently) when the transition is illegal.
func (e *ExecutionState) Transition(intentID string, expect, next ExecState) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	cur, ok := e.states[intentID]
	if !ok {
		cur = ExecPending
	}
	if cur != expect {
		return false
	}
	if !legalTransitions[cur][next] {
		return false
	}
	e.states[intentID] = next
	return true
}

func (e *ExecutionState) Forcibly(intentID string, next ExecState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.states[intentID] = next
}

func (e *ExecutionState) Reset(intentID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.states, intentID)
}