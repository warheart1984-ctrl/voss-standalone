package main

import (
	"testing"
)

// Conformance: every Lambda law has an enforcement stage in the GRE-1001
// pipeline and surfaces a named rule_ref. This is the auditability trace
// matrix: latent -> state -> gate path -> ledger evidence.

func mappingLambdaToStage() map[LambdaLaw]string {
	return map[LambdaLaw]string{
		Lambda1: StageInputValidation,
		Lambda2: StageAuditTrail,
		Lambda3: StageInputValidation,
		Lambda4: StageIdentitySeparation,
		Lambda5: StageDriftCheck,
		Lambda6: StageOperatorCorrigibility,
		Lambda7: StageUSLGate,
	}
}

func TestConformanceEveryLambdaMapsToStage(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	gre := r.gre
	seen := map[string]bool{}
	for _, s := range gre.Stages {
		seen[s.Name] = true
	}
	for law, stage := range mappingLambdaToStage() {
		if _, ok := seen[stage]; !ok {
			t.Fatalf("law %s maps to stage %s which is not present in GRE-1001", law, stage)
		}
	}
	// Stage order is fixed: input -> capability -> identity -> usl -> drift ->
	// corrigibility -> audit -> ledger -> output.
	order := []string{StageInputValidation, StageCapabilityCheck, StageIdentitySeparation,
		StageUSLGate, StageDriftCheck, StageOperatorCorrigibility, StageAuditTrail,
		StageLedgerWrite, StageOutputValidation}
	for i, want := range order {
		if i >= len(gre.Stages) || gre.Stages[i].Name != want {
			t.Fatalf("stage %d: expected %s, got %v", i, want, stageNameOr(gre.Stages, i))
		}
	}
}

func stageNameOr(stages []StageStep, i int) string {
	if i < len(stages) {
		return stages[i].Name
	}
	return "<missing>"
}

func TestConformanceAdmitTraceMatrix(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
r.RegisterProvider("fake", NewFakeAdapter(true))

	req := testRequest()
	out, err := r.Admit(req)
	if err != nil {
		t.Fatalf("admit err: %v", err)
	}
	if out.Decision.Result != Admit {
		t.Fatalf("expected ADMIT, got %s (%s)", out.Decision.Result, out.Decision.Reason)
	}

// Every audit claim has ledger evidence.
	byStage := map[string]StageRecord{}
	for _, s := range out.Stages {
		byStage[s.Stage] = s
	}
	for law, stage := range mappingLambdaToStage() {
		rec, ok := byStage[stage]
		if !ok {
			t.Fatalf("no audit record for %s (%s)", law, stage)
		}
		if !rec.Passed {
			t.Fatalf("audit record failed: %s %s", rec.Stage, rec.Reason)
		}
		if rec.RuleRef == "" {
			t.Fatalf("stage %s must carry a rule_ref for auditability", stage)
		}
	}
	// The identity and operator stages must name their laws explicitly.
	if byStage[StageIdentitySeparation].RuleRef != string(Lambda4) {
		t.Fatalf("identity stage must reference Lambda.4, got %s", byStage[StageIdentitySeparation].RuleRef)
	}
	if byStage[StageOperatorCorrigibility].RuleRef != string(Lambda6) {
		t.Fatalf("corrigibility stage must reference Lambda.6, got %s", byStage[StageOperatorCorrigibility].RuleRef)
	}

	if out.Entry.Result != Admit || out.Entry.Admitted != true {
		t.Fatalf("ledger entry must record ADMIT/admitted, got %s/%v", out.Entry.Result, out.Entry.Admitted)
	}
	if out.Entry.ReplayID == "" {
		t.Fatal("ledger entry must carry a replay marker")
	}
	if out.Entry.Hash == "" || out.Entry.PrevHash == "genesis" && out.Entry.Index != 0 {
		t.Fatal("ledger entry must be chained beyond genesis")
	}
}

func TestConformanceDenyHasNamedEvidence(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	req := testRequest()
	req.CapabilityClass.Class = "image" // not granted -> capability.lattice
	out, err := r.Admit(req)
	if err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	if out.Decision.Result != Deny {
		t.Fatalf("expected DENY for ungranted capability, got %s", out.Decision.Result)
	}
	if out.Decision.RuleRef == "" {
		t.Fatal("DENY must carry a rule_ref for auditability")
	}
	// The failure must be ledgered with the same rule_ref.
	if out.Entry.RuleRef != out.Decision.RuleRef {
		t.Fatalf("ledger rule_ref mismatch: %s vs %s", out.Entry.RuleRef, out.Decision.RuleRef)
	}
}

func TestConformanceReplayAddressability(t *testing.T) {
	// Lambda.1: same input + same state -> same admission decision.
	r := NewRouter(testTenants(), testLattice())
r.RegisterProvider("fake", NewFakeAdapter(true))
	req := testRequest()

	o1, err1 := r.Admit(req)
	o2, err2 := r.Admit(req)
	if err1 != nil || err2 != nil {
		t.Fatalf("admit errors: %v %v", err1, err2)
	}
	if o1.Decision.Result != o2.Decision.Result || o1.Decision.RuleRef != o2.Decision.RuleRef {
		t.Fatalf("determinism violated across replays: %+v vs %+v", o1.Decision, o2.Decision)
	}
}

func TestConformanceLambdaFailClosedCases(t *testing.T) {
	tenants := testTenants()

	cases := []struct {
		name     string
		mutate   func(*CapabilityRequest)
		law      LambdaLaw
		expected bool
	}{
		{name: "lambda3 no lane", mutate: func(r *CapabilityRequest) { r.MLCALane = "" }, law: Lambda3, expected: false},
		{name: "lambda6 no operator", mutate: func(r *CapabilityRequest) { r.OperatorID = "" }, law: Lambda6, expected: false},
		{name: "lambda4 wrong tenant", mutate: func(r *CapabilityRequest) { r.TenantID = "tenant-b" }, law: Lambda4, expected: false},
		{name: "lambda2 no tenant", mutate: func(r *CapabilityRequest) { r.TenantID = "" }, law: Lambda2, expected: false},
	}
	for _, c := range cases {
		r := testRequest()
		c.mutate(&r)
		gotLambda(t, r, tenants, c.law, c.expected, c.name)
	}
}

func gotLambda(t *testing.T, req CapabilityRequest, tenants []TenantPolicy, law LambdaLaw, want bool, name string) {
	t.Helper()
	var got bool
	switch law {
	case Lambda1:
		got = checkLambda1(req).Passed
	case Lambda2:
		got = checkLambda2(req).Passed
	case Lambda3:
		got = checkLambda3(req).Passed
	case Lambda4:
		got = checkLambda4(req, tenants).Passed
	case Lambda5:
		got = checkLambda5().Passed
	case Lambda6:
		got = checkLambda6(req).Passed
	case Lambda7:
		got = checkLambda7(true).Passed
	}
	if got != want {
		t.Fatalf("%s: %s expected passed=%v, got %v", name, law, want, got)
	}
}
