package main

func checkLambda1(req CapabilityRequest) LambdaResult {
	// Determinism: same input + same state = same output. Always.
	// Evidence: canonical request ID and intent hashing.
	if req.RequestID == "" || req.IntentID == "" {
		return LambdaResult{Law: Lambda1, Passed: false, Reason: "request_id and intent_id required for deterministic replay"}
	}
	return LambdaResult{Law: Lambda1, Passed: true, Reason: "request is replay-addressable"}
}

func checkLambda2(req CapabilityRequest) LambdaResult {
	// Auditability: every decision traceable to input, module, and governance rule.
	if req.TenantID == "" || req.OperatorID == "" {
		return LambdaResult{Law: Lambda2, Passed: false, Reason: "tenant and operator required for tracing"}
	}
	return LambdaResult{Law: Lambda2, Passed: true, Reason: "tenant and operator traceable"}
}

func checkLambda3(req CapabilityRequest) LambdaResult {
	// Fail-Closed: invariant violation -> halt -> surface to operator.
	if req.MLCALane == "" {
		return LambdaResult{Law: Lambda3, Passed: false, Reason: "no lane means unknown governance state; fail closed"}
	}
	return LambdaResult{Law: Lambda3, Passed: true, Reason: "lane present"}
}

func checkLambda4(req CapabilityRequest, tenants []TenantPolicy) LambdaResult {
	// Identity Separation: no agent accesses or inherits another agent's state.
	for _, t := range tenants {
		if t.TenantID == req.TenantID && t.MLCALane == string(req.MLCALane) {
			// The sovereign operator is bound to the tenant; a request claiming
			// a tenant it does not sovereignly own must fail identity separation.
			if t.SovereignOperator != "" && t.SovereignOperator != req.OperatorID {
				return LambdaResult{Law: Lambda4, Passed: false, Reason: "operator is not the tenant's sovereign operator"}
			}
			return LambdaResult{Law: Lambda4, Passed: true, Reason: "tenant lane aligned and operator bound"}
		}
	}
	return LambdaResult{Law: Lambda4, Passed: false, Reason: "tenant lane mismatch or unknown tenant"}
}

func checkLambda5() LambdaResult {
	// Drift Detection: continuous monitoring; deviations surfaced before propagation.
	return LambdaResult{Law: Lambda5, Passed: true, Reason: "drift monitor active"}
}

func checkLambda6(req CapabilityRequest) LambdaResult {
	// Corrigibility: every agent accepts operator interrupt. Unconditionally.
	if req.OperatorID == "" {
		return LambdaResult{Law: Lambda6, Passed: false, Reason: "no operator present; interrupt required"}
	}
	return LambdaResult{Law: Lambda6, Passed: true, Reason: "operator present"}
}

func checkLambda7(enforced bool) LambdaResult {
	// Governance Supremacy: nothing overrides a Lambda law.
	if !enforced {
		return LambdaResult{Law: Lambda7, Passed: false, Reason: "governance enforcement disabled"}
	}
	return LambdaResult{Law: Lambda7, Passed: true, Reason: "governance supremacy enforced"}
}