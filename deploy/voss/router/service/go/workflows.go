package main

import (
	"errors"
	"fmt"
)

// WorkflowID identifies one of the four governed workflows from
// deploy/voss/workflow/*.yaml.
type WorkflowID string

const (
	WorkflowGovernedIntake       WorkflowID = "governed-intake"
	WorkflowOTEMExecution        WorkflowID = "otem-execution"
	WorkflowExternalSuggestion   WorkflowID = "external-suggestion-admission"
	WorkflowGovernedTrainingEval WorkflowID = "governed-training-eval"
)

// WorkflowSpec binds a workflow to a capability and the lanes it may run in.
// Every step that reaches an adapter goes through GovernAndExecute: USL gate,
// GRE-1001 pipeline, Immune Protocol, and operator precedence before the call.
type WorkflowSpec struct {
	ID              WorkflowID
	CapabilityClass CapabilityClass
	AllowedLanes    []MLCALane
	Steps           []string
}

func (w WorkflowSpec) Allows(lane MLCALane) bool {
	for _, l := range w.AllowedLanes {
		if l == lane {
			return true
		}
	}
	return false
}

// WorkflowService is the governed entry point for the four workflows. It owns
// the mapping from workflow -> capability and enforces per-workflow lane
// restrictions before anything reaches the router.
type WorkflowService struct {
	router  *Router
	specs   map[WorkflowID]WorkflowSpec
	bubbles []string
}

func NewWorkflowService(r *Router) *WorkflowService {
	return &WorkflowService{
		router: r,
		specs: map[WorkflowID]WorkflowSpec{
			WorkflowGovernedIntake: {
				ID:              WorkflowGovernedIntake,
				CapabilityClass: CapabilityClass{Class: "intake", Scope: "unstructured", Action: "ingest", Risk: "low"},
				AllowedLanes:    []MLCALane{LaneSafe, LaneNormal, LaneExpress},
				Steps:           []string{"receive", "classify", "ledger"},
			},
			WorkflowOTEMExecution: {
				ID:              WorkflowOTEMExecution,
				CapabilityClass: CapabilityClass{Class: "execution", Scope: "otem", Action: "execute", Risk: "medium"},
				AllowedLanes:    []MLCALane{LaneNormal, LaneExpress},
				Steps:           []string{"proposal", "approval", "preview", "verify", "apply", "ledger"},
			},
			WorkflowExternalSuggestion: {
				ID:              WorkflowExternalSuggestion,
				CapabilityClass: CapabilityClass{Class: "admission", Scope: "external", Action: "attach", Risk: "low"},
				AllowedLanes:    []MLCALane{LaneSafe, LaneNormal},
				Steps:           []string{"ingest", "sanitize", "admit_as_evidence", "ledger"},
			},
			WorkflowGovernedTrainingEval: {
				ID:              WorkflowGovernedTrainingEval,
				CapabilityClass: CapabilityClass{Class: "training", Scope: "eval", Action: "run", Risk: "low"},
				AllowedLanes:    []MLCALane{LaneSafe},
				Steps:           []string{"plan", "data_gate", "train", "eval", "ledger"},
			},
		},
	}
}

func (ws *WorkflowService) Spec(id WorkflowID) (WorkflowSpec, bool) {
	s, ok := ws.specs[id]
	return s, ok
}

// Run admits and executes one workflow intent. The only adapter contact is
// inside GovernAndExecute; a denied or operator-overridden intent is never
// executed and never leaves a provider result in the ledger.
func (ws *WorkflowService) Run(id WorkflowID, req CapabilityRequest) (AdmissionOutcome, ExecutionResult, error) {
	spec, ok := ws.specs[id]
	if !ok {
		return AdmissionOutcome{}, ExecutionResult{}, fmt.Errorf("unknown workflow %q", id)
	}
	if !spec.Allows(req.MLCALane) {
		return AdmissionOutcome{}, ExecutionResult{}, fmt.Errorf("workflow %q not allowed in lane %s", id, req.MLCALane)
	}
	// Bind the workflow's governed capability onto the request.
	req.CapabilityClass = spec.CapabilityClass
	return ws.router.GovernAndExecute(req)
}

var ErrWorkflowLaneBlocked = errors.New("workflow lane not permitted for capability")