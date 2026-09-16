package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	tenantsPath := "G:\\Project Finish\\Voss Standalone\\Voss Standalone\\deploy\\voss\\config\\tenants.production.json"
	cfg := LoadTenants(tenantsPath)
	router := NewRouterWithConfig(cfg)

	// Register example adapters - no default provider enabled by default
	// router.RegisterProvider("nvidia-nim", NewNvidiaNIMAdapter("http://nvidia-nim:8000","key-ref-nim"))
	// router.RegisterProvider("generic", NewGenericAdapter("generic"))

	http.HandleFunc("/route", router.Route)
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Voss Model Router listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Router struct {
	providers map[string]Adapter
	usl       USLGate
	ledger    *Ledger
}

func NewRouterWithConfig(cfg TenantsConfig) *Router {
	return &Router{
		providers: make(map[string]Adapter),
		usl:       NewUSLGateWithTenants(cfg),
		ledger:    NewLedger(),
	}
}

func (r *Router) RegisterProvider(id string, a Adapter) {
	r.providers[id] = a
}

func (r *Router) Route(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	var cr CapabilityRequest
	if err := json.NewDecoder(req.Body).Decode(&cr); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	admitted, reason := r.usl.Check(cr)
	if !admitted {
		incAdmission("rejected")
		r.ledger.WriteDecision(cr, false)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"admitted": false, "reason": reason})
		decisionDuration.WithLabelValues("rejected").Observe(time.Since(start).Seconds())
		return
	}
	adapter, ok := r.providers[cr.ModelRef.ProviderID]
	if !ok {
		incAdmission("rejected")
		r.ledger.WriteDecision(cr, false)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"admitted": false, "reason": "no enabled provider"})
		decisionDuration.WithLabelValues("rejected").Observe(time.Since(start).Seconds())
		return
	}
	if ok, reason := adapter.CapabilityRequest(cr); !ok {
		incProviderError(cr.ModelRef.ProviderID)
		incAdmission("rejected")
		r.ledger.WriteDecision(cr, false)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"admitted": false, "reason": reason})
		decisionDuration.WithLabelValues("rejected").Observe(time.Since(start).Seconds())
		return
	}
	incAdmission("admitted")
	r.ledger.WriteDecision(cr, true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"admitted": true, "provider": cr.ModelRef.ProviderID})
	decisionDuration.WithLabelValues("admitted").Observe(time.Since(start).Seconds())
}
