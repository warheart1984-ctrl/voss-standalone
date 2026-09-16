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
}

func NewServer(r *Router) *Server { return &Server{router: r} }

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
	if err == ErrInterrupted {
		status = http.StatusConflict
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decision": outcome.Decision,
		"ledger":   outcome.Entry,
		"request":  req,
	})
}

type interruptRequest struct {
	IntentID   string `json:"intent_id"`
	OperatorID string `json:"operator_id"`
}

func (s *Server) InterruptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ir interruptRequest
	if err := json.NewDecoder(r.Body).Decode(&ir); err != nil || ir.IntentID == "" || ir.OperatorID == "" {
		http.Error(w, "intent_id and operator_id required", http.StatusBadRequest)
		return
	}
	s.router.Interrupts().Interrupt(ir.IntentID)
	s.router.Ledger().WriteDecision(CapabilityRequest{IntentID: ir.IntentID, TenantID: "operator", RequestID: "interrupt"}, false, Deny, "operator interrupt", string(Lambda6), fmt.Sprintf("int-%d", time.Now().UnixNano()), []StageRecord{{Stage: "operator_corrigibility", Passed: false, Reason: "operator interrupt", RuleRef: string(Lambda6)}})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"result": "interrupted", "intent_id": ir.IntentID})
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