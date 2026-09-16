package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestExecutionStateTransitions(t *testing.T) {
	e := NewExecutionState()
	if e.Current("i") != ExecPending {
		t.Fatalf("expected PENDING, got %s", e.Current("i"))
	}
	if !e.Transition("i", ExecPending, ExecAdmitted) {
		t.Fatal("PENDING->ADMITTED should be legal")
	}
	if !e.Transition("i", ExecAdmitted, ExecRunning) {
		t.Fatal("ADMITTED->RUNNING should be legal")
	}
	if e.Transition("i", ExecRunning, ExecAdmitted) {
		t.Fatal("RUNNING->ADMITTED must be illegal")
	}
	if !e.Transition("i", ExecRunning, ExecHalted) {
		t.Fatal("RUNNING->HALTED should be legal")
	}
	if !e.Current("i").IsTerminal() {
		t.Fatal("HALTED must be terminal")
	}
	if e.Transition("i", ExecHalted, ExecRunning) {
		t.Fatal("terminal HALTED must reject all transitions")
	}
	e.Forcibly("i", ExecTerminated)
	if e.Current("i") != ExecTerminated {
		t.Fatalf("expected TERMINATED, got %s", e.Current("i"))
	}
	if !e.Current("i").IsTerminal() {
		t.Fatal("TERMINATED must be terminal")
	}
}

func TestOperatorAuth(t *testing.T) {
	a := NewOperatorAuth("")
	if a.Authorized("Bearer x") {
		t.Fatal("empty token must fail closed")
	}
	b := NewOperatorAuth("secret")
	if !b.Authorized("Bearer secret") {
		t.Fatal("correct token must pass")
	}
	if b.Authorized("bearer secret") {
		t.Fatal("lowercase scheme must be rejected")
	}
	if b.Authorized("Bearer wrong") {
		t.Fatal("wrong token must fail")
	}
}

func TestIntegrationUnauthorizedOperator(t *testing.T) {
	s := integrationRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/interrupt", strings.NewReader(`{"intent_id":"x","operator_id":"op-admin"}`))
	s.InterruptHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestIntegrationCorrection(t *testing.T) {
	s := integrationRouter(t)
	admit := `{"request_id":"c1","tenant_id":"demo-tenant","mlca_lane":"SAFE","capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},"intent_id":"ci1","operator_id":"op-admin","model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},"payload":"hello"}`

	if code, _ := postJSON(t, s, "/route", admit); code != http.StatusOK {
		t.Fatalf("baseline admit expected 200, got %d", code)
	}
	code, _ := postJSON(t, s, "/correct", `{"intent_id":"ci1","operator_id":"op-admin","directive":"halt and re-plan the training set"}`)
	if code != http.StatusOK {
		t.Fatalf("correction expected 200, got %d", code)
	}
	code, out := postJSON(t, s, "/route", admit)
	if code != http.StatusForbidden {
		t.Fatalf("corrected re-admit expected 403, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("corrected re-admit expected DENY, got %s", got)
	}
	if got := decisionOf(t, out, "rule_ref"); got != string(Lambda6) {
		t.Fatalf("corrected re-admit expected Lambda.6, got %s", got)
	}
}

func TestIntegrationTerminationIsTerminal(t *testing.T) {
	s := integrationRouter(t)
	admit := `{"request_id":"t1","tenant_id":"demo-tenant","mlca_lane":"SAFE","capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},"intent_id":"ti1","operator_id":"op-admin","model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},"payload":"hello"}`

	code, _ := postJSON(t, s, "/terminate", `{"intent_id":"ti1","operator_id":"op-admin"}`)
	if code != http.StatusOK {
		t.Fatalf("termination expected 200, got %d", code)
	}

	// Subsequent attempts for the same intent deny as terminated (409), forever.
	for i := 0; i < 3; i++ {
		code, out := postJSON(t, s, "/route", admit)
		if code != http.StatusConflict {
			t.Fatalf("iter %d: terminated intent expected 409, got %d", i, code)
		}
		if got := decisionOf(t, out, "result"); got != string(Deny) {
			t.Fatalf("iter %d: expected DENY, got %s", i, got)
		}
	}

	if st := s.router.Execution().Current("ti1"); st != ExecTerminated {
		t.Fatalf("expected TERMINATED state, got %s", st)
	}
}

func TestIntegrationTerminateIdempotentLedgered(t *testing.T) {
	s := integrationRouter(t)
	postJSON(t, s, "/terminate", `{"intent_id":"ti2","operator_id":"op-admin"}`)
	postJSON(t, s, "/terminate", `{"intent_id":"ti2","operator_id":"op-admin"}`)
	for _, en := range s.router.Ledger().Export() {
		if en.IntentID != "ti2" {
			continue
		}
		if en.Result != Deny || en.RuleRef != string(Lambda6) {
			t.Fatalf("operator override entry must be DENY Lambda.6, got %v %v", en.Result, en.RuleRef)
		}
	}
}

func TestInterruptWinsDuringInFlight(t *testing.T) {
	cfg := LoadConfig("../../../config/tenants.production.json")
	tenants, lattice := cfg.Build()
	r := NewRouter(tenants, lattice)

	// A slow adapter whose CapabilityRequest blocks long enough for the
	// interrupt to land while the request is mid-flight through the router.
	slow := NewSlowBlockingAdapter()
	r.RegisterProvider("project-infinity", slow)

	req := testReqForInfo("i1")
	outcomeCh := make(chan struct {
		out AdmissionOutcome
		err error
	}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := r.Admit(req)
		outcomeCh <- struct {
			out AdmissionOutcome
			err error
		}{out, err}
	}()

	// Wait until the adapter has been invoked, then interrupt mid-flight.
	slow.Called()
	r.Interrupts().Interrupt(req.IntentID)
	close(slow.release)

	wg.Wait()
	res := <-outcomeCh

	if res.err != ErrInterrupted {
		t.Fatalf("expected ErrInterrupted from in-flight admit, got %v", res.err)
	}
	if res.out.Decision.Result != Deny {
		t.Fatalf("expected DENY for interrupted in-flight admit, got %v", res.out.Decision.Result)
	}
	if st := r.Execution().Current(req.IntentID); st != ExecHalted {
		t.Fatalf("expected HALTED execution state, got %s", st)
	}
	if slow.observedGenerate {
		t.Fatalf("provider Generate must never be reached for an interrupted intent")
	}
}

func testReqForInfo(intentID string) CapabilityRequest {
	return CapabilityRequest{
		RequestID:       "req-" + intentID,
		TenantID:        "demo-tenant",
		MLCALane:        LaneSafe,
		CapabilityClass: CapabilityClass{Class: "inference", Scope: "low_risk", Action: "execute", Risk: "low"},
		IntentID:        intentID,
		OperatorID:      "op-admin",
		ModelRef:        ModelRef{ProviderID: "project-infinity", ModelID: "m", KeyRef: "k"},
		Payload:         []byte("hello"),
	}
}

// SlowBlockingAdapter blocks inside CapabilityRequest until released, modeling
// an in-flight provider round-trip; Generate is never reachable on the Admit
// path but the field lets tests assert it was not invoked.
type SlowBlockingAdapter struct {
	release          chan struct{}
	calledOnce       chan struct{}
	observedGenerate bool
	mu               sync.Mutex
}

func NewSlowBlockingAdapter() *SlowBlockingAdapter {
	return &SlowBlockingAdapter{
		release:    make(chan struct{}),
		calledOnce: make(chan struct{}, 1),
	}
}

func (a *SlowBlockingAdapter) CapabilityRequest(req CapabilityRequest) (bool, string) {
	select {
	case a.calledOnce <- struct{}{}:
	default:
	}
	<-a.release
	return true, ""
}

// Called blocks until the adapter's CapabilityRequest has been entered.
func (a *SlowBlockingAdapter) Called() {
	<-a.calledOnce
}

func (a *SlowBlockingAdapter) Generate(req CapabilityRequest) (map[string]interface{}, error) {
	a.mu.Lock()
	a.observedGenerate = true
	a.mu.Unlock()
	return map[string]interface{}{"ok": true}, nil
}

func (a *SlowBlockingAdapter) Trace(req CapabilityRequest) (map[string]interface{}, error) {
	return map[string]interface{}{"trace_id": req.IntentID}, nil
}