package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	decisionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "voss_router_decision_duration_seconds",
		Help: "Router decision duration",
	}, []string{"result"})
	
	admissionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_router_admission_total",
		Help: "Total admissions",
	}, []string{"result"})
	
	providerErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_router_provider_error_total",
		Help: "Provider errors",
	}, []string{"provider"})
)

func incAdmission(result string) { admissionTotal.WithLabelValues(result).Inc() }
func incProviderError(provider string) { providerErrorTotal.WithLabelValues(provider).Inc() }
