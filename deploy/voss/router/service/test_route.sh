#!/bin/bash
curl -X POST http://localhost:8080/route \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "demo-tenant",
    "mlca_lane": "SAFE",
    "capability_class": "inference.low_risk",
    "intent_id": "test-intent-001",
    "operator_id": "op-admin",
    "model_ref": {
      "provider_id": "project-infinity-cog",
      "model_id": "infinity-cog-v1",
      "key_ref": "key-ref-infinity"
    }
  }'
