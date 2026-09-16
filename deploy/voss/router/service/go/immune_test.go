package main

import "testing"

func TestImmuneUnknownTenantReject(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	req.TenantID = ""
	d := i.Classify(req)
	if d.Result != ImmuneReject {
		t.Fatalf("expected REJECT, got %s", d.Result)
	}
}

func TestImmuneEmptyPayloadQuarantine(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	req.Payload = nil
	d := i.Classify(req)
	if d.Result != ImmuneQuarantine {
		t.Fatalf("expected QUARANTINE, got %s", d.Result)
	}
}

func TestImmuneSafeAllow(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	d := i.Classify(req)
	if d.Result != ImmuneAllow {
		t.Fatalf("expected ALLOW, got %s", d.Result)
	}
}

func TestImmuneNormalClamp(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	req.MLCALane = LaneNormal
	d := i.Classify(req)
	if d.Result != ImmuneClamp {
		t.Fatalf("expected CLAMP, got %s", d.Result)
	}
}

func TestImmuneExpressReroute(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	req.MLCALane = LaneExpress
	d := i.Classify(req)
	if d.Result != ImmuneReroute {
		t.Fatalf("expected REROUTE, got %s", d.Result)
	}
}

func TestImmuneUnknownLaneReject(t *testing.T) {
	i := NewImmuneProtocol()
	req := testRequest()
	req.MLCALane = "BOGUS"
	d := i.Classify(req)
	if d.Result != ImmuneReject {
		t.Fatalf("expected REJECT, got %s", d.Result)
	}
}