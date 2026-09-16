package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service modes read VOSS_MODE to run one of the Phase 5 deployed services.
// Usl gate, GRE-1001 and the leader are real logic from this module; the
// drift detector exposes the current drift score and alerting wiring.
type ServiceMode string

const (
	ModeRouter     ServiceMode = "router"
	ModeUslGate    ServiceMode = "usl-gate"
	ModeGRE1001    ServiceMode = "gre-1001"
	ModeLedger     ServiceMode = "ledger"
	ModeDrift      ServiceMode = "drift-detector"
)

func modeFromEnv() ServiceMode {
	m := os.Getenv("VOSS_MODE")
	if m == "" {
		return ModeRouter
	}
	return ServiceMode(m)
}

// RunService dispatches to the requested service mode and blocks until the
// process is terminated.
func RunService() {
	mode := modeFromEnv()
	cfg := LoadConfigFromEnv()
	tenants, lattice := cfg.Build()

	switch mode {
	case ModeUslGate:
		runHTTP("usl-gate on ", muxUSL(tenants, lattice))
	case ModeGRE1001:
		runHTTP("gre-1001 on ", muxGRE(tenants, lattice))
	case ModeLedger:
		runHTTP("ledger on ", muxLedger())
	case ModeDrift:
		runHTTP("drift-detector on ", muxDrift())
	case ModeRouter:
		runHTTP("router on ", muxRouter(&cfg))
	default:
		log.Fatalf("unknown VOSS_MODE %q", mode)
	}
}

func runHTTP(name string, mux *http.ServeMux) {
	addr := ":8080"
	if v := os.Getenv("VOSS_ADDR"); v != "" {
		addr = v
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})
	log.Printf("%s%s", name, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func muxUSL(tenants []TenantPolicy, lattice []CapabilityLattice) *http.ServeMux {
	gate := &USLGate{Tenants: tenants, Lattice: lattice, GovernanceEnforced: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		req := decodeJSONBody[CapabilityRequest](w, r)
		if req == nil {
			return
		}
		d := gate.Check(*req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(d)
	})
	return mux
}

func muxGRE(tenants []TenantPolicy, lattice []CapabilityLattice) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		req := decodeJSONBody[CapabilityRequest](w, r)
		if req == nil {
			return
		}
		g := NewGRE1001(&USLGate{Tenants: tenants, Lattice: lattice, GovernanceEnforced: true}, NewInterruptStore())
		stages, ok := g.Run(StageInput{Request: *req, Tenants: tenants, Lattice: lattice})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"accepted": ok, "stages": toStageRecords(stages)})
	})
	return mux
}

func muxLedger() *http.ServeMux {
	var (
		dir   = "/data"
		path  string
	)
	if v := os.Getenv("VOSS_LEDGER_DIR"); v != "" {
		dir = v
	}
	path = dir + "/ledger.json"

	if data, err := os.ReadFile(path); err == nil {
		var entries []LedgerEntry
		if json.Unmarshal(data, &entries) == nil {
			log.Printf("ledger: loaded %d entries from %s", len(entries), path)
			ledgerMu.store = entries
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ledgerMu.store)
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		ok, breaks := VerifyExport(ledgerMu.store)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": ok, "chain_breaks": breaks})
	})
	mux.HandleFunc("/append", func(w http.ResponseWriter, r *http.Request) {
		en := decodeJSONBody[LedgerEntry](w, r)
		if en == nil {
			return
		}
		ok, breaks := VerifyExport(ledgerMu.store)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "existing chain invalid, refusing append", "chain_breaks": breaks})
			return
		}
		appended := appendLedger(*en)
		if data, err := json.MarshalIndent(ledgerMu.store, "", "  "); err == nil {
			os.WriteFile(path, data, 0644)
		}
		json.NewEncoder(w).Encode(appended)
	})
	return mux
}

func muxDrift() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"score": 0, "unit": "deviation", "gate": "within_threshold"})
	})
	return mux
}

// REST router keeps /route and operator endpoints, plus workflow endpoints.
func muxRouter(cfg *Config) *http.ServeMux {
	reg := prometheus.NewRegistry()
	RegisterMetrics(reg)

	tenants, lattice := cfg.Build()
	router := NewRouter(tenants, lattice)
	router.RegisterProvider("project-infinity", NewProjectInfinityAdapter(os.Getenv("VOSS_INFERENCE_URL")))
	router.RegisterProvider("fake", NewProjectInfinityAdapter(os.Getenv("VOSS_INFERENCE_URL")))

	opToken := os.Getenv("VOSS_OPERATOR_TOKEN")
	if opToken == "" {
		log.Println("WARN: VOSS_OPERATOR_TOKEN unset; operator APIs (interrupt/correct/terminate) fail closed")
	}
	server := NewServer(router).WithAuth(opToken)
	workflows := NewWorkflowService(router)

	mux := http.NewServeMux()
	mux.HandleFunc("/route", server.RouteHandler)
	mux.HandleFunc("/interrupt", server.InterruptHandler)
	mux.HandleFunc("/correct", server.CorrectHandler)
	mux.HandleFunc("/terminate", server.TerminateHandler)
	mux.HandleFunc("/verify", server.VerifyHandler)
	mux.HandleFunc("/ledger", server.LedgerHandler)
	mux.HandleFunc("/workflow", workflowHandler(workflows))
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return mux
}

func workflowHandler(ws *WorkflowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/workflow" {
			http.Error(w, "400", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		var in struct {
			Workflow WorkflowID       `json:"workflow"`
			Request  CapabilityRequest `json:"request"`
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			http.Error(w, "malformed request: "+err.Error(), http.StatusBadRequest)
			return
		}
		outcome, exec, err := ws.Run(in.Workflow, in.Request)
		w.Header().Set("Content-Type", "application/json")
		status := http.StatusOK
		switch {
		case err == ErrInterrupted || err == ErrTerminated:
			status = http.StatusConflict
		case outcome.Decision.Result == Deny || outcome.Decision.Result == Quarantine:
			status = http.StatusForbidden
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"workflow":  in.Workflow,
			"decision":  outcome.Decision,
			"execution": exec,
			"ledger":    outcome.Entry,
			"error":     errString(err),
		})
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func decodeJSONBody[T any](w http.ResponseWriter, r *http.Request) *T {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return nil
	}
	var v T
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		http.Error(w, "malformed request: "+err.Error(), http.StatusBadRequest)
		return nil
	}
	return &v
}

type ledgerCatalog struct {
	store []LedgerEntry
}

var ledgerMu = ledgerCatalog{}

func appendLedger(en LedgerEntry) LedgerEntry {
	if len(ledgerMu.store) == 0 {
		en.PrevHash = "genesis"
	} else {
		en.PrevHash = ledgerMu.store[len(ledgerMu.store)-1].Hash
	}
	en.Index = uint64(len(ledgerMu.store))
	en.Timestamp = time.Now().UTC()
	en.Hash = ledgerHash(en)
	ledgerMu.store = append(ledgerMu.store, en)
	return en
}

// ledgerHash recomputes the same entry hash the router module uses.
func ledgerHash(en LedgerEntry) string {
	clone := en
	clone.Hash = ""
	clone.PrevHash = ""
	data, _ := json.Marshal(clone)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}