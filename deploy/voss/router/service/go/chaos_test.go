package main

import (
	"errors"
	"testing"
	"time"
)

// Dishamory chaos suite: after a governance fault, no unexpected continuation
// is allowed; every fault either denies with evidence or halts with reason.

func chaosRouter(t *testing.T) *Router {
	t.Helper()
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))
	return r
}

func TestChaosProviderFailClosesNoContinuation(t *testing.T) {
	r := chaosRouter(t)
	failing := &errAdapter{err: errors.New("provider timeout after 2500ms")}
	r.RegisterProvider("slow", failing)

	req := testRequest()
	req.ModelRef.ProviderID = "slow"

	out, exec, err := r.GovernAndExecute(req)
	if err != nil {
		t.Fatalf("provider failure must be a governed DENY, not an error: %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY on provider failure, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef != "provider.execution" {
		t.Fatalf("expected provider.execution rule ref, got %s", out.Decision.RuleRef)
	}
	if exec.Hash != "" {
		t.Fatal("failed provider must not produce an execution result")
	}
	// The provider was already admitted but failed to execute: no successful
	// execution hash may reach the ledger.
	for _, en := range r.Ledger().Export() {
		if en.ExecutionHash != "" {
			t.Fatalf("unexpected execution hash in ledger after provider failure: %s", en.ExecutionHash)
		}
	}
	verified, breaks := r.Ledger().Verify()
	if !verified || breaks != 0 {
		t.Fatalf("ledger must remain intact: %d breaks", breaks)
	}
}

func TestChaosDriftSpikeHaltsBeforeProvider(t *testing.T) {
	r := chaosRouter(t)
	r.gre.SteerDrift(0.91)

	req := testRequest()
	out, exec, err := r.GovernAndExecute(req)
	if err != nil {
		t.Fatalf("drift spike is a governed DENY, not an error: %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY on drift spike, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef != string(Lambda5) {
		t.Fatalf("expected Lambda.5 rule ref, got %s", out.Decision.RuleRef)
	}
	if exec.Hash != "" {
		t.Fatal("drift spike must stop before provider execution")
	}
	sawDrift := false
	for _, s := range out.Stages {
		if s.Stage == StageDriftCheck && !s.Passed {
			sawDrift = true
		}
	}
	if !sawDrift {
		t.Fatal("halt stage record must include failing drift check")
	}
}

func TestChaosLedgerBreakFailsClosed(t *testing.T) {
	r := chaosRouter(t)

	// Establish a clean chain, then break it by forging an entry.
	if _, err := r.Admit(testRequest()); err != nil {
		t.Fatalf("admit: %v", err)
	}
	r.ledger.mu.Lock()
	r.ledger.entries = append(r.ledger.entries, LedgerEntry{IntentID: "forged", PrevHash: "attacker"})
	r.ledger.nextIdx++
	r.ledger.mu.Unlock()

	if ok, breaks := r.ledger.Verify(); ok || breaks == 0 {
		t.Fatalf("expected broken chain, ok=%v breaks=%d", ok, breaks)
	}

	out, err := r.Admit(testRequest())
	if !errors.Is(err, ErrLedgerBreak) {
		t.Fatalf("expected ErrLedgerBreak, got %v", err)
	}
	if out.Decision.Result != Deny || out.Decision.RuleRef != "ledger.integrity" {
		t.Fatalf("expected DENY ledger.integrity, got %s %s", out.Decision.Result, out.Decision.RuleRef)
	}
}

func TestChaosMalformedInputQuarantined(t *testing.T) {
	r := chaosRouter(t)
	req := testRequest()
	req.Payload = nil
	out, err := r.Admit(req)
	if err != nil {
		t.Fatalf("malformed input is a governed QUARANTINE, not an error: %v", err)
	}
	if out.Decision.Result != Quarantine {
		t.Fatalf("expected QUARANTINE for empty payload, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef != "immune.protocol" {
		t.Fatalf("expected immune.protocol, got %s", out.Decision.RuleRef)
	}
}

func TestChaosUSLOutageLambda7FailsClosed(t *testing.T) {
	gate := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: false}
	req := testRequest()
	d := gate.Check(req)
	// Must deny because governance enforcement is off, never continue.
	if d.Result != Deny || d.RuleRef != string(Lambda7) {
		t.Fatalf("expected DENY Lambda.7 on USL outage, got %s %s", d.Result, d.RuleRef)
	}
}

func TestChaosOperatorInterruptHaltsExecution(t *testing.T) {
	r := chaosRouter(t)
	req := testRequest()
	r.GovernAndExecute(req)

	r.Interrupts().Interrupt(req.IntentID)
	r.Execution().Forcibly(req.IntentID, ExecHalted)

	out, _, err := r.GovernAndExecute(req)
	if err != ErrInterrupted {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY, got %s", out.Decision.Result)
	}
	if st := r.Execution().Current(req.IntentID); st != ExecHalted {
		t.Fatalf("expected HALTED, got %s", st)
	}
}

func TestChaosSlowProviderBounded(t *testing.T) {
	r := chaosRouter(t)
	old := ProviderTimeout
	ProviderTimeout = 20 * time.Millisecond
	defer func() { ProviderTimeout = old }()

	slow := &slowProvider{wait: 200 * time.Millisecond}
	r.RegisterProvider("slowp", slow)

	req := testRequest()
	req.ModelRef.ProviderID = "slowp"
	start := time.Now()
	out, exec, err := r.GovernAndExecute(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("bounded slow provider must not error: %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY for provider timeout policy, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef != "provider.execution" {
		t.Fatalf("expected provider.execution rule ref for timeout, got %s", out.Decision.RuleRef)
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("slow provider must be bounded, took %v", elapsed)
	}
	if exec.Hash != "" {
		t.Fatal("timed-out provider must not produce an execution result")
	}
}

// errAdapter fails Generate immediately.
type errAdapter struct{ err error }

func (a *errAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) { return true, "" }
func (a *errAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	return nil, a.err
}
func (a *errAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}

// slowProvider sleeps past any reasonable deadline; the router must bound it.
type slowProvider struct{ wait time.Duration }

func (a *slowProvider) CapabilityRequest(req CapabilityRequest) (bool, string) { return true, "" }
func (a *slowProvider) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	time.Sleep(a.wait)
	return map[string]interface{}{"ok": true}, nil
}
func (a *slowProvider) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}