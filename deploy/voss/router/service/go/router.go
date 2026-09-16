package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrNoProvider = errors.New("no enabled provider for capability")
var ErrInterrupted = errors.New("operator interruption")
var ErrTerminated = errors.New("operator termination")

type Router struct {
	usl        *USLGate
	gre        *GRE1001
	immune     *ImmuneProtocol
	ledger     *Ledger
	interrupts *InterruptStore
	exec       *ExecutionState
	providers  map[string]Adapter
}

func NewRouter(tenants []TenantPolicy, lattice []CapabilityLattice) *Router {
	usl := &USLGate{Tenants: tenants, Lattice: lattice, GovernanceEnforced: true}
	interrupts := NewInterruptStore()
	return &Router{
		usl:        usl,
		gre:        NewGRE1001(usl, interrupts),
		immune:     NewImmuneProtocol(),
		ledger:     NewLedger(),
		interrupts: interrupts,
		exec:       NewExecutionState(),
		providers:  make(map[string]Adapter),
	}
}

func (r *Router) RegisterProvider(id string, a Adapter) { r.providers[id] = a }

func (r *Router) Ledger() *Ledger { return r.ledger }

func (r *Router) Interrupts() *InterruptStore { return r.interrupts }

func (r *Router) Execution() *ExecutionState { return r.exec }

type AdmissionOutcome struct {
	Decision AdmissionDecision
	Entry    LedgerEntry
	Stages   []StageRecord
}

func (r *Router) Admit(req CapabilityRequest) (AdmissionOutcome, error) {
	start := time.Now()
	defer func() { MetricCycleLatency.Observe(time.Since(start).Seconds()) }()

	// Operator interrupt and termination have precedence (Lambda.6).
	if r.interrupts.IsTerminated(req.IntentID) {
		MetricInterrupts.Inc()
		decision := admittedDecision(req, Deny, "operator termination", string(Lambda6), decID())
		r.exec.Forcibly(req.IntentID, ExecTerminated)
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, []StageRecord{{Stage: StageOperatorCorrigibility, Passed: false, Reason: "operator termination", RuleRef: string(Lambda6)}})
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ErrTerminated
	}
	if r.interrupts.IsInterrupted(req.IntentID) {
		MetricInterrupts.Inc()
		MetricInterruptLatency.Observe(time.Since(start).Seconds())
		decision := admittedDecision(req, Deny, "operator interruption", string(Lambda6), decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, []StageRecord{{Stage: StageOperatorCorrigibility, Passed: false, Reason: "operator interrupt", RuleRef: string(Lambda6)}})
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ErrInterrupted
	}

	// GRE-1001 pipeline: all admission logic before any provider call.
	stages, ok := r.gre.Run(StageInput{Request: req, Tenants: r.usl.Tenants, Lattice: r.usl.Lattice})
	stageRecords := toStageRecords(stages)
	if !ok {
		result := Deny
		last := stages[len(stages)-1]
		decision := admittedDecision(req, result, last.Reason, last.RuleRef, decID())
		entry := r.ledger.WriteDecision(req, false, result, decision.Reason, decision.RuleRef, decision.DecisionID, stageRecords)
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(result)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, nil
	}

	// Immune Protocol must run before any provider call.
	immune := r.immune.Classify(req)
	if immune.Result != ImmuneAllow {
		decision := admittedDecision(req, Quarantine, "immune: "+immune.Reason, "immune.protocol", decID())
		entry := r.ledger.WriteDecision(req, false, Quarantine, decision.Reason, decision.RuleRef, decision.DecisionID, append(stageRecords, StageRecord{Stage: "immune", Passed: false, Reason: immune.Reason, RuleRef: "immune.protocol"}))
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Quarantine)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, nil
	}

	// Explicit provider lookup - no default provider.
	adapter, ok := r.providers[req.ModelRef.ProviderID]
	if !ok {
		decision := admittedDecision(req, Deny, "no enabled provider for capability", "provider.registry", decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, append(stageRecords, StageRecord{Stage: "provider", Passed: false, Reason: "no provider registered", RuleRef: "provider.registry"}))
		MetricProviderErrors.WithLabelValues(req.ModelRef.ProviderID).Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		MetricLedgerWrites.Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, nil
	}

	// Adapter capability check.
	if ok, reason := adapter.CapabilityRequest(req); !ok {
		decision := admittedDecision(req, Deny, "adapter: "+reason, "provider.adapter", decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, append(stageRecords, StageRecord{Stage: "provider", Passed: false, Reason: reason, RuleRef: "provider.adapter"}))
		MetricProviderErrors.WithLabelValues(req.ModelRef.ProviderID).Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		MetricLedgerWrites.Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, nil
	}

	// In-flight operator precedence: an interrupt or correction issued while
	// this request was passing gates (pre-admit race) must still win.
	if r.interrupts.IsTerminated(req.IntentID) {
		MetricInterrupts.Inc()
		r.exec.Forcibly(req.IntentID, ExecTerminated)
		decision := admittedDecision(req, Deny, "operator termination during flight", string(Lambda6), decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, append(stageRecords, StageRecord{Stage: "provider", Passed: false, Reason: "operator termination during flight", RuleRef: string(Lambda6)}))
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ErrTerminated
	}
	if r.interrupts.IsInterrupted(req.IntentID) {
		MetricInterrupts.Inc()
		MetricInterruptLatency.Observe(time.Since(start).Seconds())
		r.exec.Forcibly(req.IntentID, ExecHalted)
		decision := admittedDecision(req, Deny, "operator interruption during flight", string(Lambda6), decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, append(stageRecords, StageRecord{Stage: "provider", Passed: false, Reason: "operator interruption during flight", RuleRef: string(Lambda6)}))
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ErrInterrupted
	}

	// Admit.
	decision := admittedDecision(req, Admit, "admitted", "usl.gate", decID())
	entry := r.ledger.WriteDecision(req, true, Admit, decision.Reason, decision.RuleRef, decision.DecisionID, stageRecords)
	MetricAdmissions.WithLabelValues(string(Admit)).Inc()
	MetricLedgerWrites.Inc()
	return AdmissionOutcome{Decision: decision, Entry: entry}, nil
}

func admittedDecision(req CapabilityRequest, result AdmitResult, reason, ruleRef, decisionID string) AdmissionDecision {
	return AdmissionDecision{
		Result:     result,
		Reason:     reason,
		RuleRef:    ruleRef,
		DecisionID: decisionID,
		RequestID:  req.RequestID,
		Timestamp:  time.Now().UTC(),
	}
}

func decID() string {
	return fmt.Sprintf("dec-%d", time.Now().UnixNano())
}

func toStageRecords(stages []StageOutput) []StageRecord {
	out := make([]StageRecord, len(stages))
	for i, s := range stages {
		out[i] = StageRecord{Stage: s.Stage, Passed: s.Passed, Reason: s.Reason, RuleRef: s.RuleRef}
	}
	return out
}

func HashInput(req CapabilityRequest) string {
	data := []byte(req.TenantID + "|" + req.IntentID + "|" + req.ModelRef.ProviderID + "|" + req.OperatorID)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}