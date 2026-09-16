package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerChainingAndVerify(t *testing.T) {
	l := NewLedger()
	req := testRequest()
	e1 := l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d1", nil)
	e2 := l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d2", nil)
	if e1.PrevHash != "genesis" {
		t.Fatalf("first entry prev_hash should be genesis, got %s", e1.PrevHash)
	}
	if e2.PrevHash != e1.Hash {
		t.Fatalf("second entry prev_hash must chain: %s != %s", e2.PrevHash, e1.Hash)
	}
	ok, breaks := l.Verify()
	if !ok || breaks != 0 {
		t.Fatalf("expected valid chain, got ok=%v breaks=%d", ok, breaks)
	}
}

func TestLedgerChainBreakDetection(t *testing.T) {
	l := NewLedger()
	req := testRequest()
	e1 := l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d1", nil)
	_ = l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d2", nil)
	// tamper: corrupt the first entry hash
	l.mu.Lock()
	l.entries[0].Hash = "tampered"
	l.mu.Unlock()
	ok, breaks := l.Verify()
	if ok || breaks == 0 {
		t.Fatalf("expected chain break to be detected")
	}
	_ = e1
}

func TestLedgerExportIndependentVerify(t *testing.T) {
	l := NewLedger()
	req := testRequest()
	l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d1", nil)
	l.WriteDecision(req, false, Deny, "tenant not found", "Lambda.4", "d2", nil)

	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := l.ExportToFile(path); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var imported []LedgerEntry
	if err := json.Unmarshal(data, &imported); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	ok, breaks := VerifyExport(imported)
	if !ok || breaks != 0 {
		t.Fatalf("independent verify failed: ok=%v breaks=%d", ok, breaks)
	}
}

func TestLedgerCERGeneration(t *testing.T) {
	l := NewLedger()
	req := testRequest()
	e := l.WriteDecision(req, true, Admit, "ok", "usl.gate", "dec-123", []StageRecord{{Stage: "usl_gate", Passed: true}})
	cer, ok := l.GenerateCER(e.DecisionID)
	if !ok {
		t.Fatalf("CER not generated")
	}
	if cer.InputHash == "" || cer.ReplayMarker == "" || cer.DecisionID != "dec-123" {
		t.Fatalf("CER missing required fields")
	}
}

func TestLedgerReplayID(t *testing.T) {
	l := NewLedger()
	req := testRequest()
	e := l.WriteDecision(req, true, Admit, "ok", "usl.gate", "d1", nil)
	if e.ReplayID == "" {
		t.Fatalf("replay marker required")
	}
}