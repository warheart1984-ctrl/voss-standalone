package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func integrationRouter(t *testing.T) *Server {
	t.Helper()
	cfg := LoadConfig("../../../config/tenants.production.json")
	if len(cfg.Tenants) == 0 {
		t.Fatal("production config must load")
	}
	tenants, lattice := cfg.Build()
	r := NewRouter(tenants, lattice)
	r.RegisterProvider("project-infinity", NewProjectInfinityAdapter("http://infinity:8000"))
	return NewServer(r).WithAuth("test-operator-token")
}

func postJSON(t *testing.T, s *Server, path, body string) (int, map[string]interface{}) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if path == "/interrupt" || path == "/correct" || path == "/terminate" {
		req.Header.Set("Authorization", "Bearer test-operator-token")
	}
	switch path {
	case "/interrupt":
		s.InterruptHandler(rr, req)
	case "/correct":
		s.CorrectHandler(rr, req)
	case "/terminate":
		s.TerminateHandler(rr, req)
	default:
		s.RouteHandler(rr, req)
	}
	var out map[string]interface{}
	if len(rr.Body.String()) > 0 {
		json.Unmarshal(rr.Body.Bytes(), &out)
	}
	return rr.Code, out
}

func decisionOf(t *testing.T, out map[string]interface{}, field string) string {
	t.Helper()
	if d, ok := out["decision"].(map[string]interface{}); ok {
		if v, ok := d[field]; ok {
			return v.(string)
		}
	}
	return ""
}

func TestIntegrationProducedConfigAdmit(t *testing.T) {
	s := integrationRouter(t)
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r1","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i1","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"hello"}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", code, out)
	}
	if got := decisionOf(t, out, "result"); got != string(Admit) {
		t.Fatalf("expected ADMIT, got %s", got)
	}
}

func TestIntegrationCapabilityNotGrantedDenied(t *testing.T) {
	s := integrationRouter(t)
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r2","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"training","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i2","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"{}"}`)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("expected DENY, got %s", got)
	}
}

func TestIntegrationLaneMismatchDenied(t *testing.T) {
	s := integrationRouter(t)
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r3","tenant_id":"demo-tenant","mlca_lane":"EXPRESS",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i3","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"{}"}`)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("expected DENY, got %s", got)
	}
	if got := decisionOf(t, out, "rule_ref"); got != string(Lambda4) {
		t.Fatalf("expected Lambda.4 rule, got %s", got)
	}
}

func TestIntegrationUnknownProviderDenied(t *testing.T) {
	s := integrationRouter(t)
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r4","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i4","operator_id":"op-admin",
		"model_ref":{"provider_id":"novendor","model_id":"m","key_ref":"k"},
		"payload":"{}"}`)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("expected DENY, got %s", got)
	}
}

func TestIntegrationUnknownTenantDenied(t *testing.T) {
	s := integrationRouter(t)
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r5","tenant_id":"ghost","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i5","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"{}"}`)
	if code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("expected DENY, got %s", got)
	}
}

func TestIntegrationMalformedJSON(t *testing.T) {
	s := integrationRouter(t)
	code, _ := postJSON(t, s, "/route", `{not json`)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestIntegrationUnknownFieldRejected(t *testing.T) {
	s := integrationRouter(t)
	code, _ := postJSON(t, s, "/route", `{
		"request_id":"int-r6","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i6","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"{}","sneaky":"extra"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 (unknown field), got %d", code)
	}
}

func TestIntegrationInterruptPrecedence(t *testing.T) {
	s := integrationRouter(t)
	postJSON(t, s, "/route", `{
		"request_id":"int-r7","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i7","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"hello"}`)
	code, _ := postJSON(t, s, "/interrupt", `{"intent_id":"int-i7","operator_id":"op-admin"}`)
	if code != http.StatusOK {
		t.Fatalf("expected interrupt 200, got %d", code)
	}
	code, out := postJSON(t, s, "/route", `{
		"request_id":"int-r8","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i7","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"hello"}`)
	if code != http.StatusConflict {
		t.Fatalf("expected 409 interrupted, got %d", code)
	}
	if got := decisionOf(t, out, "result"); got != string(Deny) {
		t.Fatalf("expected DENY on interrupted intent, got %s", got)
	}
}

func TestIntegrationLedgerAndVerify(t *testing.T) {
	s := integrationRouter(t)
	postJSON(t, s, "/route", `{
		"request_id":"int-r9","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"inference","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i9","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"hello"}`)
	postJSON(t, s, "/route", `{
		"request_id":"int-r10","tenant_id":"demo-tenant","mlca_lane":"SAFE",
		"capability_class":{"class":"training","scope":"low_risk","action":"execute","risk":"low"},
		"intent_id":"int-i10","operator_id":"op-admin",
		"model_ref":{"provider_id":"project-infinity","model_id":"m","key_ref":"k"},
		"payload":"{}"}`)

	rr := httptest.NewRecorder()
	s.LedgerHandler(rr, httptest.NewRequest(http.MethodGet, "/ledger", nil))
	var entries []LedgerEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("ledger decode: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	if entries[0].Result != Admit || entries[1].Result != Deny {
		t.Fatalf("unexpected ledger results: %v, %v", entries[0].Result, entries[1].Result)
	}

	rr = httptest.NewRecorder()
	s.VerifyHandler(rr, httptest.NewRequest(http.MethodGet, "/verify", nil))
	var v struct {
		Valid       bool `json:"valid"`
		ChainBreaks int  `json:"chain_breaks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("verify decode: %v", err)
	}
	if !v.Valid || v.ChainBreaks != 0 {
		t.Fatalf("expected valid chain, got valid=%v breaks=%d", v.Valid, v.ChainBreaks)
	}
}