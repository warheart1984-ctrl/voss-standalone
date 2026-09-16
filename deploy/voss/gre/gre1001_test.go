package main

import "testing"

func TestGRE1001Pass(t *testing.T) {
	gre := NewGRE1001()
	req := CapabilityRequest{TenantID:"t1", IntentID:"i1", OperatorID:"op1"}
	ok, _ := gre.Run(req)
	if !ok { t.Fatalf("expected pass") }
}
