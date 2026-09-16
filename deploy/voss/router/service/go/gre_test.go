package main

import "testing"

func TestGREAllStagesPass(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	si := StageInput{Request: testRequest(), Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if !ok {
		t.Fatalf("expected full pass, failed at %s: %s", tailReason(stages), tailReason(stages))
	}
	if len(stages) != 9 {
		t.Fatalf("expected 9 stages, got %d", len(stages))
	}
}

func TestGREInputValidationFails(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.RequestID = ""
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on missing request_id")
	}
	if len(stages) == 0 || stages[len(stages)-1].Stage != StageInputValidation {
		t.Fatalf("expected halt at input_validation")
	}
}

func TestGRECapabilityCheckFails(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.CapabilityClass = CapabilityClass{Class: "inference"}
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on incomplete capability")
	}
	if stages[len(stages)-1].Stage != StageCapabilityCheck {
		t.Fatalf("expected halt at capability_check")
	}
}

func TestGREIdentitySeparationFails(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.MLCALane = LaneExpress
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on identity separation")
	}
	if stages[len(stages)-1].Stage != StageIdentitySeparation {
		t.Fatalf("expected halt at identity_separation, got %s", stages[len(stages)-1].Stage)
	}
}

func TestGREUSLGateFails(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.CapabilityClass = CapabilityClass{Class: "training", Scope: "default", Action: "execute", Risk: "low"}
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on un-granted capability")
	}
	if stages[len(stages)-1].Stage != StageUSLGate {
		t.Fatalf("expected halt at usl_gate, got %s", stages[len(stages)-1].Stage)
	}
}

func TestGROperatorCorrigibilityFailsOnInterrupt(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	interrupts := NewInterruptStore()
	gre := NewGRE1001(usl, interrupts)
	req := testRequest()
	interrupts.Interrupt(req.IntentID)
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on operator interrupt")
	}
	if stages[len(stages)-1].Stage != StageOperatorCorrigibility {
		t.Fatalf("expected halt at operator_corrigibility, got %s", stages[len(stages)-1].Stage)
	}
}

func TestGRENoOperatorFailsAtInput(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.OperatorID = ""
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok {
		t.Fatalf("expected halt on missing operator")
	}
	if stages[len(stages)-1].Stage != StageInputValidation {
		t.Fatalf("expected halt at input_validation on missing operator, got %s", stages[len(stages)-1].Stage)
	}
}

func TestGREHaltAndSurface(t *testing.T) {
	usl := &USLGate{Tenants: testTenants(), Lattice: testLattice(), GovernanceEnforced: true}
	gre := NewGRE1001(usl, NewInterruptStore())
	req := testRequest()
	req.TenantID = "ghost"
	si := StageInput{Request: req, Tenants: testTenants(), Lattice: testLattice()}
	stages, ok := gre.Run(si)
	if ok || len(stages) == 0 {
		t.Fatalf("expected halt-and-surface")
	}
	if stages[len(stages)-1].Passed {
		t.Fatalf("last stage must fail")
	}
}

func tailReason(stages []StageOutput) string {
	if len(stages) == 0 {
		return "unknown"
	}
	return stages[len(stages)-1].Reason
}