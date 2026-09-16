package main

import "testing"

func TestUSLAdmit(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	d := usl.Check(testRequest())
	if d.Result != Admit {
		t.Fatalf("expected ADMIT, got %s: %s", d.Result, d.Reason)
	}
}

func TestUSLUnknownTenant(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	req := testRequest()
	req.TenantID = "ghost"
	d := usl.Check(req)
	if d.Result != Deny || d.RuleRef != string(Lambda4) {
		t.Fatalf("expected DENY by Lambda.4, got %s rule=%s", d.Result, d.RuleRef)
	}
}

func TestUSLLaneMismatch(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	req := testRequest()
	req.MLCALane = LaneExpress
	d := usl.Check(req)
	if d.Result != Deny || d.RuleRef != string(Lambda4) {
		t.Fatalf("expected DENY by Lambda.4 on lane mismatch, got %s rule=%s reason=%s", d.Result, d.RuleRef, d.Reason)
	}
}

func TestUSLCapabilityNotGranted(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	req := testRequest()
	req.CapabilityClass = CapabilityClass{Class: "training", Scope: "default", Action: "execute", Risk: "low"}
	d := usl.Check(req)
	if d.Result != Deny {
		t.Fatalf("expected DENY for un-granted capability, got %s", d.Result)
	}
}

func TestUSLTenantPolicyRejects(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	req := testRequest()
	req.CapabilityClass = CapabilityClass{Class: "inference", Scope: "default", Action: "execute", Risk: "high"}
	d := usl.Check(req)
	if d.Result != Deny {
		t.Fatalf("expected DENY for disallowed risk, got %s", d.Result)
	}
}

func TestUSLMissingOperator(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	req := testRequest()
	req.OperatorID = ""
	d := usl.Check(req)
	if d.Result != Deny || d.RuleRef != string(Lambda6) {
		t.Fatalf("expected DENY by Lambda.6, got %s rule=%s", d.Result, d.RuleRef)
	}
}

func TestUSLGovernanceDisabled(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: false}
	d := usl.Check(testRequest())
	if d.Result != Deny || d.RuleRef != string(Lambda7) {
		t.Fatalf("expected DENY by Lambda.7 when governance disabled, got %s rule=%s", d.Result, d.RuleRef)
	}
}

func TestUSLDecisionIDPresent(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	d := usl.Check(testRequest())
	if d.DecisionID == "" || d.Timestamp.IsZero() {
		t.Fatalf("decision must carry decision_id and timestamp")
	}
}