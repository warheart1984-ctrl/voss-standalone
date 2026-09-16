package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Server struct {
	router *Router
	auth   *OperatorAuth
}

func NewServer(r *Router) *Server {
	return &Server{router: r, auth: NewOperatorAuth("")}
}

func (s *Server) WithAuth(token string) *Server {
	s.auth = NewOperatorAuth(token)
	return s
}

func (s *Server) authOperator(w http.ResponseWriter, r *http.Request) bool {
	if !s.auth.Authorized(r.Header.Get("Authorization")) {
		http.Error(w, "unauthorized operator", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) RouteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	var req CapabilityRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "malformed request: "+err.Error(), http.StatusBadRequest)
		return
	}
	outcome, err := s.router.Admit(req)
	w.Header().Set("Content-Type", "application/json")
	status := http.StatusOK
	if outcome.Decision.Result == Deny || outcome.Decision.Result == Quarantine {
		status = http.StatusForbidden
	}
	if err == ErrInterrupted || err == ErrTerminated {
		status = http.StatusConflict
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decision": outcome.Decision,
		"ledger":   outcome.Entry,
		"request":  req,
	})
}

type operatorRequest struct {
	IntentID   string `json:"intent_id"`
	OperatorID string `json:"operator_id"`
	Directive  string `json:"directive,omitempty"`
}

func (s *Server) InterruptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authOperator(w, r) {
		return
	}
	var ir operatorRequest
	if err := json.NewDecoder(r.Body).Decode(&ir); err != nil || ir.IntentID == "" || ir.OperatorID == "" {
		http.Error(w, "intent_id and operator_id required", http.StatusBadRequest)
		return
	}
	s.router.Interrupts().Interrupt(ir.IntentID)
	s.router.Execution().Forcibly(ir.IntentID, ExecHalted)
	s.operatorLedger(ir, "operator interrupt", string(Lambda6))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"result": "interrupted", "intent_id": ir.IntentID})
}

func (s *Server) CorrectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authOperator(w, r) {
		return
	}
	var cr operatorRequest
	if err := json.NewDecoder(r.Body).Decode(&cr); err != nil || cr.IntentID == "" || cr.OperatorID == "" || cr.Directive == "" {
		http.Error(w, "intent_id, operator_id and directive required", http.StatusBadRequest)
		return
	}
	s.router.Interrupts().Correct(cr.IntentID, cr.Directive)
	s.router.Execution().Forcibly(cr.IntentID, ExecHalted)
	s.operatorLedger(cr, "operator correction: "+cr.Directive, string(Lambda6))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"result": "corrected", "intent_id": cr.IntentID})
}

func (s *Server) TerminateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authOperator(w, r) {
		return
	}
	var tr operatorRequest
	if err := json.NewDecoder(r.Body).Decode(&tr); err != nil || tr.IntentID == "" || tr.OperatorID == "" {
		http.Error(w, "intent_id and operator_id required", http.StatusBadRequest)
		return
	}
	s.router.Interrupts().Terminate(tr.IntentID)
	s.router.Execution().Forcibly(tr.IntentID, ExecTerminated)
	s.operatorLedger(tr, "operator termination", string(Lambda6))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"result": "terminated", "intent_id": tr.IntentID})
}

func (s *Server) operatorLedger(or operatorRequest, reason, ruleRef string) {
	s.router.Ledger().WriteDecision(
		CapabilityRequest{IntentID: or.IntentID, TenantID: "operator", RequestID: "operator"},
		false, Deny, reason, ruleRef, fmt.Sprintf("op-%d", time.Now().UnixNano()),
		[]StageRecord{{Stage: "operator_corrigibility", Passed: false, Reason: reason, RuleRef: ruleRef}},
	)
}

func (s *Server) LedgerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.router.Ledger().Export())
}

func (s *Server) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	ok, breaks := s.router.Ledger().Verify()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"valid": ok, "chain_breaks": breaks})
}