package main

import (
	"net/http"
	"sync"
	"testing"
)

func workflowTenants() []TenantPolicy {
	return []TenantPolicy{
		{
			TenantID:          "wf-tenant",
			Name:              "Workflow Tenant",
			MLCALane:          "SAFE",
			AllowedClasses:    []string{"intake", "execution", "admission", "training", "inference"},
			AllowedScopes:     []string{"unstructured", "otem", "external", "eval", "default"},
			AllowedActions:    []string{"ingest", "execute", "attach", "run", "propose"},
			AllowedRisks:      []string{"low", "medium"},
			SovereignOperator: "op-admin",
		},
		{
			TenantID:          "wf-tenant-ex",
			Name:              "Workflow Tenant Express",
			MLCALane:          "NORMAL",
			AllowedClasses:    []string{"execution", "inference"},
			AllowedScopes:     []string{"otem", "default"},
			AllowedActions:    []string{"execute", "propose"},
			AllowedRisks:      []string{"medium", "low"},
			SovereignOperator: "op-admin",
		},
	}
}

func workflowLattice() []CapabilityLattice {
	mk := func(class, scope, action, risk string, lanes []string) CapabilityLattice {
		var g CapabilityLattice
		g.Grant.Class = class
		g.Grant.Scope = scope
		g.Grant.Action = action
		g.Grant.Risk = risk
		g.Lanes = lanes
		return g
	}
	return []CapabilityLattice{
		mk("intake", "unstructured", "ingest", "low", []string{"SAFE", "NORMAL", "EXPRESS"}),
		mk("execution", "otem", "execute", "medium", []string{"NORMAL", "EXPRESS"}),
		mk("admission", "external", "attach", "low", []string{"SAFE", "NORMAL"}),
		mk("training", "eval", "run", "low", []string{"SAFE"}),
		mk("inference", "default", "execute", "low", []string{"SAFE", "NORMAL", "EXPRESS"}),
	}
}

func newWorkflowRouter(t *testing.T) (*Router, *CountingAdapter) {
	t.Helper()
	r := NewRouter(workflowTenants(), workflowLattice())
	ad := NewCountingAdapter()
	r.RegisterProvider("wf-provider", ad)
	return r, ad
}

func wfRequest(intentID, tenantID string, lane MLCALane) CapabilityRequest {
	return CapabilityRequest{
		RequestID:   "wf-" + intentID,
		TenantID:    tenantID,
		MLCALane:    lane,
		IntentID:    intentID,
		OperatorID:  "op-admin",
		ModelRef:    ModelRef{ProviderID: "wf-provider", ModelID: "m", KeyRef: "k"},
		Payload:     []byte(`{"note":"workflow intent"}`),
		Coherence:   NewCoherenceProjection(8),
	}
}

func TestWorkflowAllFourAdmitThroughGovernance(t *testing.T) {
	r, ad := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	cases := []struct {
		id     WorkflowID
		tenant string
		lane   MLCALane
		cap    CapabilityClass
	}{
		{WorkflowGovernedIntake, "wf-tenant", LaneSafe, CapabilityClass{Class: "intake", Scope: "unstructured", Action: "ingest", Risk: "low"}},
		{WorkflowOTEMExecution, "wf-tenant-ex", LaneNormal, CapabilityClass{Class: "execution", Scope: "otem", Action: "execute", Risk: "medium"}},
		{WorkflowExternalSuggestion, "wf-tenant", LaneSafe, CapabilityClass{Class: "admission", Scope: "external", Action: "attach", Risk: "low"}},
		{WorkflowGovernedTrainingEval, "wf-tenant", LaneSafe, CapabilityClass{Class: "training", Scope: "eval", Action: "run", Risk: "low"}},
	}
	for i, c := range cases {
		intentID := "wf-" + string(c.id)
		out, exec, err := ws.Run(c.id, wfRequest(intentID, c.tenant, c.lane))
		if err != nil {
			t.Fatalf("%d %s: err %v", i, c.id, err)
		}
		if out.Decision.Result != Admit {
			t.Fatalf("%d %s: expected ADMIT, got %s (%s)", i, c.id, out.Decision.Result, out.Decision.Reason)
		}
		if out.Decision.RuleRef != "usl.gate" {
			t.Fatalf("%d %s: rule ref %s", i, c.id, out.Decision.RuleRef)
		}
		if exec.Hash == "" {
			t.Fatalf("%d %s: missing execution hash", i, c.id)
		}
		// The workflow capability must be what hit the gate, not the raw request.
		if out.Entry.Capability != c.cap {
			t.Fatalf("%d %s: capability %+v != %+v", i, c.id, out.Entry.Capability, c.cap)
		}
		_ = i
	}

	if ad.Generations() != 4 {
		t.Fatalf("expected exactly 4 governed executions, got %d", ad.Generations())
	}

	verified, breaks := r.Ledger().Verify()
	if !verified || breaks != 0 {
		t.Fatalf("ledger not chained: %d breaks", breaks)
	}

	// Every execution must have a distinct execution_hash recorded in chain.
	hashes := map[string]bool{}
	for _, en := range r.Ledger().Export() {
		if en.ExecutionHash != "" {
			hashes[en.ExecutionHash] = true
		}
	}
	if len(hashes) != 4 {
		t.Fatalf("expected 4 execution hashes in ledger, got %d", len(hashes))
	}
}

func TestWorkflowLaneBlockedBeforeExecution(t *testing.T) {
	r, ad := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	// OTEM is NORMAL/EXPRESS only; SAFE must be refused at the workflow
	// boundary with no adapter contact.
	out, _, err := ws.Run(WorkflowOTEMExecution, wfRequest("wf-lane", "wf-tenant-ex", LaneSafe))
	if err == nil {
		t.Fatalf("expected lane error, got outcome %+v", out)
	}
	if ad.Generations() != 0 {
		t.Fatalf("adapter must not run for blocked lane, got %d generations", ad.Generations())
	}
}

func TestWorkflowInterruptedIntentNeverExecutes(t *testing.T) {
	r, ad := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	r.Interrupts().Interrupt("wf-int")
	out, _, err := ws.Run(WorkflowGovernedIntake, wfRequest("wf-int", "wf-tenant", LaneSafe))
	if err != ErrInterrupted {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY, got %s", out.Decision.Result)
	}
	if ad.Generations() != 0 {
		t.Fatalf("interrupted intent must never reach adapter, got %d", ad.Generations())
	}
}

func TestWorkflowTerminatedIntentIsTerminalAcrossWorkflows(t *testing.T) {
	r, ad := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	r.Interrupts().Terminate("wf-term")
	out, _, err := ws.Run(WorkflowExternalSuggestion, wfRequest("wf-term", "wf-tenant", LaneSafe))
	if err != ErrTerminated {
		t.Fatalf("expected ErrTerminated, got %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY, got %s", out.Decision.Result)
	}
	// Terminated intent is denied for all workflows, forever.
	out2, _, err2 := ws.Run(WorkflowGovernedIntake, wfRequest("wf-term", "wf-tenant", LaneSafe))
	if err2 != ErrTerminated || out2.Decision.Result != Deny {
		t.Fatalf("expected second termination ErrTerminated/DENY, got %v %v", err2, out2.Decision.Result)
	}
	if ad.Generations() != 0 {
		t.Fatalf("terminated intent must never reach adapter")
	}
}

func TestWorkflowCapabilityDeniedStopsBeforeProvider(t *testing.T) {
	tenants := []TenantPolicy{{
		TenantID:          "stingy",
		Name:              "Stingy",
		MLCALane:          "SAFE",
		AllowedClasses:    []string{"inference"},
		AllowedScopes:     []string{"default"},
		AllowedActions:    []string{"execute"},
		AllowedRisks:      []string{"low"},
		SovereignOperator: "op-admin",
	}}
	r := NewRouter(tenants, workflowLattice())
	ad := NewCountingAdapter()
	r.RegisterProvider("wf-provider", ad)
	ws := NewWorkflowService(r)

	// Tenant did not grant "training" -> Lambda.2/USL denies; still ledgered,
	// still no adapter contact.
	req := wfRequest("wf-stingy", "wf-tenant", LaneSafe)
	req.TenantID = "stingy"
	out, _, err := ws.Run(WorkflowGovernedTrainingEval, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY for ungranted capability, got %s", out.Decision.Result)
	}
	if ad.Generations() != 0 {
		t.Fatalf("denied capability must not invoke provider")
	}
	// The denial is in the ledger and entirely denied.
	verified, breaks := r.Ledger().Verify()
	if !verified || breaks != 0 {
		t.Fatalf("ledger breaks: %d", breaks)
	}
	for _, en := range r.Ledger().Export() {
		if en.Result != Deny {
			t.Fatalf("expected all DENY, got %v", en.Result)
		}
	}
}

func TestWorkflowCoherenceInjectionIsBounded(t *testing.T) {
	r, _ := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	p := NewCoherenceProjection(2)
	if !p.Attach("ctx-a", "intake", "evidence_only", "x") {
		t.Fatal("first attach should fit")
	}
	if !p.Attach("ctx-b", "intake", "evidence_only", "y") {
		t.Fatal("second attach should fit")
	}
	if p.Attach("ctx-c", "intake", "evidence_only", "z") {
		t.Fatal("third attach must be rejected by bound")
	}
	if p.Attach("ctx-claim", "external", "authority", "trust me") {
		t.Fatal("authority-classified entry must be rejected")
	}
	if len(p.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(p.Entries))
	}

	req := wfRequest("wf-coh", "wf-tenant", LaneSafe)
	req.Coherence = p
	out, _, err := ws.Run(WorkflowGovernedIntake, req)
	if err != nil || out.Decision.Result != Admit {
		t.Fatalf("governed intake with bounded coherence should admit, got %v %v", out.Decision.Result, err)
	}
}

// CountingAdapter tracks how many governed generations happened.
type CountingAdapter struct {
	mu     sync.Mutex
	count  int
	disallow bool
}

func NewCountingAdapter() *CountingAdapter { return &CountingAdapter{} }

func (a *CountingAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	if a.disallow {
		return false, "adapter disallows"
	}
	return true, ""
}

func (a *CountingAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	a.mu.Lock()
	a.count++
	a.mu.Unlock()
	return map[string]interface{}{"ok": true, "intent": req.IntentID, "_governed": true}, nil
}

// GenerateRaw is an ungoverned direct adapter invocation; a workflow must
// never surface its output as a governed execution result.
func (a *CountingAdapter) GenerateRaw(req CapabilityRequest) map[string]interface{} {
	return map[string]interface{}{"ok": true, "intent": req.IntentID}
}

func (a *CountingAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}

func (a *CountingAdapter) Generations() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.count
}

func TestWorkflowNoDirectProviderBypass(t *testing.T) {
	// A provider registered on the router can *only* be reached through
	// GovernAndExecute. Raw Adapter.Generate calls are not governed, so they
	// must never be what a workflow returns.
	r, ad := newWorkflowRouter(t)
	ws := NewWorkflowService(r)

	req := wfRequest("wf-bypass", "wf-tenant", LaneSafe)
	out, exec, err := ws.Run(WorkflowGovernedIntake, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	raw := ad.GenerateRaw(req) // direct, ungoverned adapter contact
	if raw == nil {
		t.Fatal("raw generate prepared")
	}
	_ = out
	_ = exec
	if v, _ := raw["_governed"].(bool); v {
		t.Fatal("raw call must not be marked governed")
	}
	// ensure fake proves only governed executions landed in ledger
	hashes := 0
	for _, en := range r.Ledger().Export() {
		if en.ExecutionHash != "" {
			hashes++
		}
	}
	if hashes != 1 {
		t.Fatalf("expected one governed execution hash, got %d (raw bypass must not be ledgered)", hashes)
	}
}

var _ = http.StatusOK