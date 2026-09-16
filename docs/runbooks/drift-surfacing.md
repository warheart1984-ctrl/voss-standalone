# Runbook: Drift Surfacing

Drift is monitored continuously (Lambda.5). A rise above the configured
threshold halts the GRE-1001 pipeline *before* any provider call; the spike is
surfaced, never propagated.

## Spike detection

- Gauge `voss_drift_deviation_score` in Prometheus; alert wiring in
  `deploy/voss/config/prometheus-alerts.yml`.
- Operators may steer the score with `VOSS_MODE=drift-detector` `POST /score`
  or via the GRE steered-drift hook (`SteerDrift`) for fault drills.

## During a spike

1. Every new route/workflow request is returned DENY with `rule_ref=Lambda.5`
   and stage `drift_check` logged as failed.
2. Prior admitted intents are untouched; drift stops new propagation.
3. No provider is contacted while the gate is red (`MetricExecutions` flat).

## Response

1. Identify the drift source (upstream model change, new eval mix).
2. Acknowledge and correct at the source, then lower the surface score below
   the 0.5 threshold.
3. Confirm `voss_drift_deviation_score` returns to 0 and new requests admit
   again.

## Invariant

> Deviations are surfaced to operators and stop propagation; they are not
> silently absorbed.