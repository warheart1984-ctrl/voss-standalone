package main

import "testing"

func TestUSLGateCapabilityAllowed(t *testing.T) {
	cfg := TenantsConfig{
		Tenants: []Tenant{{TenantID:"t1", MLCALane:"SAFE", CapabilityPolicy:Policy{AllowedCapabilityClasses:[]string{"inference.low_risk"}}}},
		Governance: Gov{USLGateEnforced:true},
	}
	usl := NewUSLGateWithTenants(cfg)
	req := CapabilityRequest{TenantID:"t1", MLCALane:"SAFE", CapabilityClass:"inference.low_risk", OperatorID:"op1", ModelRef:ModelRef{ProviderID:"p1"}}
	ok, _ := usl.Check(req)
	if !ok { t.Fatalf("expected admit") }
}

func TestUSLGateCapabilityDenied(t *testing.T) {
	cfg := TenantsConfig{
		Tenants: []Tenant{{TenantID:"t1", MLCALane:"SAFE", CapabilityPolicy:Policy{AllowedCapabilityClasses:[]string{"inference.low_risk"}}}},
		Governance: Gov{USLGateEnforced:true},
	}
	usl := NewUSLGateWithTenants(cfg)
	req := CapabilityRequest{TenantID:"t1", MLCALane:"SAFE", CapabilityClass:"inference.high_risk", OperatorID:"op1", ModelRef:ModelRef{ProviderID:"p1"}}
	ok, _ := usl.Check(req)
	if ok { t.Fatalf("expected reject") }
}
