package main

import "github.com/prometheus/client_golang/prometheus"

var (
	MetricCycleLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "voss_router_decision_duration_seconds",
		Help:    "Total cycle latency from request to admission decision",
		Buckets: prometheus.DefBuckets,
	})
	MetricAdmissions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_router_admission_total",
		Help: "Total admission decisions by result",
	}, []string{"result"})
	MetricProviderErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_router_provider_error_total",
		Help: "Provider errors by provider",
	}, []string{"provider"})
	MetricStageDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "voss_gre_stage_duration_seconds",
		Help:    "GRE-1001 stage duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"stage"})
	MetricUSLViolations = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_usl_violation_total",
		Help: "USL Gate invariant violations",
	})
	MetricDriftScore = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "voss_drift_deviation_score",
		Help: "Current drift deviation score",
	})
	MetricHalt = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_gre_halt_total",
		Help: "GRE-1001 pipeline halts",
	})
	MetricInterrupts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_operator_interrupt_total",
		Help: "Operator interrupt events",
	})
	MetricInterruptLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "voss_operator_interrupt_latency_seconds",
		Help:    "Time from interrupt to execution halt",
		Buckets: prometheus.DefBuckets,
	})
	MetricLedgerWrites = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_ledger_write_total",
		Help: "Ledger entries written",
	})
	MetricLedgerChainBreaks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_ledger_chain_invalid_total",
		Help: "Ledger chain integrity violations",
	})
	MetricIdentityViolations = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "voss_identity_cross_access_total",
		Help: "Identity separation violations",
	})
	MetricImmuneClass = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_immune_classification_total",
		Help: "Immune Protocol classifications",
	}, []string{"classification"})
	MetricLambdaCheck = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "voss_lambda_check_total",
		Help: "Lambda law check results",
	}, []string{"law", "passed"})
)

func RegisterMetrics(reg prometheus.Registerer) {
	for _, c := range []prometheus.Collector{
		MetricCycleLatency, MetricAdmissions, MetricProviderErrors,
		MetricStageDuration, MetricUSLViolations, MetricDriftScore,
		MetricHalt, MetricInterrupts, MetricInterruptLatency,
		MetricLedgerWrites, MetricLedgerChainBreaks, MetricIdentityViolations,
		MetricImmuneClass, MetricLambdaCheck,
	} {
		reg.MustRegister(c)
	}
}
