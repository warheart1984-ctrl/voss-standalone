package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	reg := prometheus.NewRegistry()
	RegisterMetrics(reg)

	cfg := LoadConfigFromEnv()
	tenants, lattice := cfg.Build()
	router := NewRouter(tenants, lattice)

	// Wire providers explicitly. No default provider enabled.
	router.RegisterProvider("project-infinity", NewProjectInfinityAdapter("http://infinity:8000"))
	server := NewServer(router)

	mux := http.NewServeMux()
	mux.HandleFunc("/route", server.RouteHandler)
	mux.HandleFunc("/interrupt", server.InterruptHandler)
	mux.HandleFunc("/ledger", server.LedgerHandler)
	mux.HandleFunc("/verify", server.VerifyHandler)
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	addr := ":8080"
	if v := os.Getenv("VOSS_ADDR"); v != "" {
		addr = v
	}
	log.Printf("Voss Standalone listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}