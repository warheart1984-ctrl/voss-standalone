package main

import "testing"

func TestLambda1PassAndFail(t *testing.T) {
	req := testRequest()
	pass := checkLambda1(req)
	if !pass.Passed {
		t.Fatalf("expected pass: %s", pass.Reason)
	}
	req.RequestID = ""
	fail := checkLambda1(req)
	if fail.Passed {
		t.Fatalf("expected fail on missing request_id")
	}
	req = testRequest()
	req.IntentID = ""
	if checkLambda1(req).Passed {
		t.Fatalf("expected fail on missing intent_id")
	}
}

func TestLambda2PassAndFail(t *testing.T) {
	req := testRequest()
	if !checkLambda2(req).Passed {
		t.Fatalf("expected pass")
	}
	req.TenantID = ""
	if checkLambda2(req).Passed {
		t.Fatalf("expected fail on missing tenant")
	}
	req = testRequest()
	req.OperatorID = ""
	if checkLambda2(req).Passed {
		t.Fatalf("expected fail on missing operator")
	}
}

func TestLambda3PassAndFail(t *testing.T) {
	req := testRequest()
	if !checkLambda3(req).Passed {
		t.Fatalf("expected pass")
	}
	req.MLCALane = ""
	if checkLambda3(req).Passed {
		t.Fatalf("expected fail on empty lane (fail closed)")
	}
}

func TestLambda4PassAndFail(t *testing.T) {
	req := testRequest()
	if !checkLambda4(req, testTenants()).Passed {
		t.Fatalf("expected pass for aligned tenant/lane")
	}
	req.MLCALane = LaneExpress
	if checkLambda4(req, testTenants()).Passed {
		t.Fatalf("expected fail on lane mismatch")
	}
	req = testRequest()
	req.TenantID = "ghost"
	if checkLambda4(req, testTenants()).Passed {
		t.Fatalf("expected fail on unknown tenant")
	}
}

func TestLambda5Pass(t *testing.T) {
	if !checkLambda5().Passed {
		t.Fatalf("drift monitor should be active")
	}
}

func TestLambda6PassAndFail(t *testing.T) {
	req := testRequest()
	if !checkLambda6(req).Passed {
		t.Fatalf("expected pass with operator present")
	}
	req.OperatorID = ""
	if checkLambda6(req).Passed {
		t.Fatalf("expected fail without operator")
	}
}

func TestLambda7PassAndFail(t *testing.T) {
	if !checkLambda7(true).Passed {
		t.Fatalf("expected pass when enforced")
	}
	if checkLambda7(false).Passed {
		t.Fatalf("expected fail when disabled")
	}
}