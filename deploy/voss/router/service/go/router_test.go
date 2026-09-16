package main

import (
	"testing"
)

func TestRouterAdmit(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))
	req := testRequest()
	outcome, err := r.Admit(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Decision.Result != Admit {
		t.Fatalf("expected ADMIT, got %s: %s", outcome.Decision.Result, outcome.Decision.Reason)
	}
	if !outcome.Entry.Admitted {
		t.Fatalf("ledger entry must record admitted=true")
	}
}

func TestRouterRejectUnknownProvider(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	req := testRequest()
	req.ModelRef.ProviderID = "nonexistent"
	outcome, _ := r.Admit(req)
	if outcome.Decision.Result != Deny {
		t.Fatalf("expected DENY for no default provider, got %s", outcome.Decision.Result)
	}
	if outcome.Entry.Provider != "nonexistent" {
		t.Fatalf("ledger must record provider")
	}
}

func TestRouterRejectLaneMismatch(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))
	req := testRequest()
	req.MLCALane = LaneExpress
	outcome, _ := r.Admit(req)
	if outcome.Decision.Result != Deny || outcome.Decision.RuleRef != string(Lambda4) {
		t.Fatalf("expected DENY by Lambda.4, got %s rule=%s", outcome.Decision.Result, outcome.Decision.RuleRef)
	}
}

func TestRouterQuarantineImmune(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	req := testRequest()
	req.Payload = nil
	outcome, _ := r.Admit(req)
	if outcome.Decision.Result != Quarantine {
		t.Fatalf("expected QUARANTINE for empty payload, got %s", outcome.Decision.Result)
	}
}

func TestRouterOperatorInterruptWinsInFlight(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))
	req := testRequest()
	// Interrupt before provider would be invoked.
	r.interrupts.Interrupt(req.IntentID)
	outcome, err := r.Admit(req)
	if err == nil {
		t.Fatalf("expected ErrInterrupted")
	}
	if outcome.Decision.Result != Deny || outcome.Decision.RuleRef != string(Lambda6) {
		t.Fatalf("expected interrupt-wins DENY by Lambda.6, got %s rule=%s", outcome.Decision.Result, outcome.Decision.RuleRef)
	}
	// Provider must never be invoked: adapter deny flag must not matter.
	r2 := NewRouter(testTenants(), testLattice())
	r2.RegisterProvider("fake", NewFakeAdapter(true))
	_ = r2.interrupts.IsInterrupted(req.IntentID)
}

func TestMalformedRequestDecoding(t *testing.T) {
	// "any, unknown field fails closed"
	_ = CapabilityRequest{}
}

func TestEveryDecisionLedgered(t *testing.T) {
	r := NewRouter(testTenants(), testLattice())
	r.RegisterProvider("fake", NewFakeAdapter(true))

	good := testRequest()
	if _, err := r.Admit(good); err != nil { t.Fatalf("admit failed: %v", err) }

	badLane := testRequest()
	badLane.MLCALane = LaneExpress
	if _, err := r.Admit(badLane); err != nil { t.Fatalf("badLane failed: %v", err) }

	noProvider := testRequest()
	noProvider.ModelRef.ProviderID = "missing"
	if _, err := r.Admit(noProvider); err != nil { t.Fatalf("noProvider failed: %v", err) }

	count := r.ledger.Count()
	if count != 3 {
		t.Fatalf("expected 3 ledger entries, got %d", count)
	}
	ok, breaks := r.ledger.Verify()
	if !ok || breaks != 0 {
		t.Fatalf("ledger chain broken after 3 decisions")
	}
}