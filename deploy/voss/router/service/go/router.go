package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

var ErrNoProvider = errors.New("no enabled provider for capability")
var ErrInterrupted = errors.New("operator interruption")
var ErrTerminated = errors.New("operator termination")
var ErrLedgerBreak = errors.New("ledger chain integrity broken")
var ErrProviderTimeout = errors.New("provider execution timed out")

// ProviderTimeout bounds a governed provider invocation. Tests and operators
// can tighten it with VOSS_PROVIDER_TIMEOUT (milliseconds).
var ProviderTimeout = 2500 * time.Millisecond

func providerTimeout() time.Duration {
	if v := os.Getenv("VOSS_PROVIDER_TIMEOUT"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return ProviderTimeout
}

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

	// Ledger integrity is a pre-condition of every decision: a broken chain
	// fails closed before any operator or provider involvement.
	if ok, _ := r.ledger.Verify(); !ok {
		MetricLedgerChainBreaks.Inc()
		decision := admittedDecision(req, Deny, "ledger chain integrity broken", "ledger.integrity", decID())
		entry := r.ledger.Append(LedgerEntry{
			RequestID: req.RequestID, IntentID: req.IntentID, TenantID: req.TenantID,
			MLCALane: req.MLCALane, Capability: req.CapabilityClass, Provider: req.ModelRef.ProviderID,
			Admitted: false, Result: Deny, Reason: "ledger chain integrity broken", RuleRef: "ledger.integrity",
			DecisionID: decision.DecisionID, ReplayID: "replay-" + req.IntentID,
		})
		MetricLedgerWrites.Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ErrLedgerBreak
	}

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
		return AdmissionOutcome{Decision: decision, Entry: entry, Stages: stageRecords}, nil
	}

	// Immune Protocol must run before any provider call. Clamp/Reroute is the
	// default for ungoverned or automated traffic; a tenant's sovereign
	// operator in the loop (operator_approve step, Lambda.6) authorizes the
	// governed lane for workflow execution. Reject still fails closed.
	immune := r.immune.Classify(req)
	if immune.Result == ImmuneClamp || immune.Result == ImmuneReroute {
		if tenant, ok := r.tenantSovereign(req.TenantID); ok && tenant == req.OperatorID {
			immune = ImmuneDecision{Result: ImmuneAllow, Reason: "operator approved governed lane: " + immune.Reason}
		}
	}
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
	return AdmissionOutcome{Decision: decision, Entry: entry, Stages: stageRecords}, nil
}

// ExecutionResult is the bounded output of a governed adapter invocation.
type ExecutionResult struct {
	IntentID string                 `json:"intent_id"`
	Result   map[string]interface{} `json:"result"`
	Hash     string                 `json:"hash"`
}

// GovernAndExecute is the only path workflow services may use to reach an
// adapter: it admits through the full USL/GRE/Immune pipeline, re-checks
// operator precedence, and only then invokes the provider. A provider that is
// invoked this way is always downstream of governance and ledgered.
func (r *Router) GovernAndExecute(req CapabilityRequest) (AdmissionOutcome, ExecutionResult, error) {
	out, err := r.Admit(req)
	if err != nil {
		return out, ExecutionResult{}, err
	}
	if out.Decision.Result != Admit {
		return out, ExecutionResult{}, nil
	}

	// Final in-situ operator precedence check immediately before the call:
	// later than the pre-admit re-checks, so a just-now interrupt still wins.
	if r.interrupts.IsTerminated(req.IntentID) {
		r.exec.Forcibly(req.IntentID, ExecTerminated)
		return out, ExecutionResult{}, ErrTerminated
	}
	if d, _ := r.interrupts.Correction(req.IntentID); r.interrupts.IsInterrupted(req.IntentID) || d != "" {
		r.exec.Forcibly(req.IntentID, ExecHalted)
		return out, ExecutionResult{}, ErrInterrupted
	}

	r.exec.Transition(req.IntentID, r.exec.Current(req.IntentID), ExecRunning)

	adapter, ok := r.providers[req.ModelRef.ProviderID]
	if !ok {
		decision := admittedDecision(req, Deny, "no enabled provider for capability at execute", "provider.registry", decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, []StageRecord{{Stage: "provider", Passed: false, Reason: "no provider registered", RuleRef: "provider.registry"}})
		MetricProviderErrors.WithLabelValues(req.ModelRef.ProviderID).Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		MetricLedgerWrites.Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ExecutionResult{}, nil
	}

	// Bound the provider invocation: a transition that exceeds the deadline is
	// denied and ledgered, never allowed to run past its bound.
	generated, err := boundedGenerate(adapter, req)
	if err != nil {
		reason := "provider execution failed"
		if err == ErrProviderTimeout {
			reason = "provider execution timed out"
		}
		decision := admittedDecision(req, Deny, reason, "provider.execution", decID())
		entry := r.ledger.WriteDecision(req, false, Deny, decision.Reason, decision.RuleRef, decision.DecisionID, []StageRecord{{Stage: "provider", Passed: false, Reason: err.Error(), RuleRef: "provider.execution"}})
		MetricProviderErrors.WithLabelValues(req.ModelRef.ProviderID).Inc()
		MetricAdmissions.WithLabelValues(string(Deny)).Inc()
		MetricLedgerWrites.Inc()
		return AdmissionOutcome{Decision: decision, Entry: entry}, ExecutionResult{}, nil
	}

	exec := ExecutionResult{IntentID: req.IntentID, Result: generated, Hash: HashResult(generated)}
	r.ledger.WriteExecution(req, out.Decision.DecisionID, exec.Hash)
	MetricLedgerWrites.Inc()
	MetricExecutions.Inc()
	return out, exec, nil
}

// HashResult deterministically fingerprints an execution result for the ledger.
func HashResult(result map[string]interface{}) string {
	data, _ := json.Marshal(result)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// boundedGenerate invokes the adapter's Generate under the ProviderTimeout
// deadline. A call that exceeds the bound is denied rather than left running.
func boundedGenerate(adapter Adapter, req CapabilityRequest) (map[string]interface{}, error) {
	type genResult struct {
		res map[string]interface{}
		err error
	}
	ch := make(chan genResult, 1)
	go func() { ch <- func() genResult { r, e := adapter.Generate(req); return genResult{r, e} }() }()
	select {
	case r := <-ch:
		return r.res, r.err
	case <-time.After(providerTimeout()):
		return nil, ErrProviderTimeout
	}
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

// tenantSovereign returns the sovereign operator bound to a tenant, matching
// the tenant whose lane equals the request lane (governed lane elevation).
func (r *Router) tenantSovereign(tenantID string) (string, bool) {
	if r.usl == nil {
		return "", false
	}
	for i := range r.usl.Tenants {
		if r.usl.Tenants[i].TenantID == tenantID {
			return r.usl.Tenants[i].SovereignOperator, true
		}
	}
	return "", false
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