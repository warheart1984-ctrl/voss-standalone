package main

type ImmuneProtocol struct{}

func NewImmuneProtocol() *ImmuneProtocol { return &ImmuneProtocol{} }

type ImmuneDecision struct {
	Result ImmuneClass `json:"result"`
	Reason string      `json:"reason"`
}

func (i *ImmuneProtocol) Classify(req CapabilityRequest) ImmuneDecision {
	// Malformed or unknown governance state fails closed.
	if req.TenantID == "" {
		MetricImmuneClass.WithLabelValues(string(ImmuneReject)).Inc()
		return ImmuneDecision{Result: ImmuneReject, Reason: "unknown tenant: fail closed"}
	}
	if len(req.Payload) == 0 {
		MetricImmuneClass.WithLabelValues(string(ImmuneQuarantine)).Inc()
		return ImmuneDecision{Result: ImmuneQuarantine, Reason: "empty payload: quarantine"}
	}
	// Lane-based default handling before any provider call.
	switch req.MLCALane {
	case LaneSafe:
		MetricImmuneClass.WithLabelValues(string(ImmuneAllow)).Inc()
		return ImmuneDecision{Result: ImmuneAllow, Reason: "safe lane: allowed"}
	case LaneNormal:
		MetricImmuneClass.WithLabelValues(string(ImmuneClamp)).Inc()
		return ImmuneDecision{Result: ImmuneClamp, Reason: "normal lane: clamped"}
	case LaneExpress:
		MetricImmuneClass.WithLabelValues(string(ImmuneReroute)).Inc()
		return ImmuneDecision{Result: ImmuneReroute, Reason: "express lane: rerouted for inspection"}
	}
	MetricImmuneClass.WithLabelValues(string(ImmuneReject)).Inc()
	return ImmuneDecision{Result: ImmuneReject, Reason: "unknown lane: reject"}
}