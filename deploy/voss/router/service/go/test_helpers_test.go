package main

func testTenants() []TenantPolicy {
	return []TenantPolicy{
		{
			TenantID:       "tenant-a",
			Name:           "Tenant A",
			MLCALane:       "SAFE",
			AllowedClasses: []string{"inference"},
			AllowedScopes:  []string{"default"},
			AllowedActions: []string{"execute"},
			AllowedRisks:   []string{"low"},
			SovereignOperator: "op-a",
		},
		{
			TenantID:          "tenant-b",
			Name:              "Tenant B",
			MLCALane:          "NORMAL",
			AllowedClasses:    []string{"inference"},
			AllowedScopes:     []string{"default"},
			AllowedActions:    []string{"execute"},
			AllowedRisks:      []string{"low", "medium"},
			SovereignOperator: "op-b",
		},
	}
}

func testLattice() []CapabilityLattice {
	g1 := CapabilityLattice{}
	g1.Grant.Class = "inference"
	g1.Grant.Scope = "default"
	g1.Grant.Action = "execute"
	g1.Grant.Risk = "low"
	g1.Lanes = []string{"SAFE", "NORMAL", "EXPRESS"}
	return []CapabilityLattice{g1}
}

func testRequest() CapabilityRequest {
	return CapabilityRequest{
		RequestID:  "req-001",
		TenantID:   "tenant-a",
		MLCALane:   LaneSafe,
		CapabilityClass: CapabilityClass{Class: "inference", Scope: "default", Action: "execute", Risk: "low"},
		IntentID:   "intent-001",
		OperatorID: "op-a",
		ModelRef:   ModelRef{ProviderID: "fake", ModelID: "fake-model", KeyRef: "key-1"},
		Payload:    []byte("hello"),
	}
}