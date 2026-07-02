#!/usr/bin/env bash
# Re-stamp the shared service templates into every app chart.
# deploy/_service-templates is the single source of truth; per-chart
# Chart.yaml/values.yaml are NOT touched. Run via `make charts`.
set -euo pipefail
cd "$(dirname "$0")"

SERVICES="bff product order inventory payment notification frontend"

for s in $SERVICES; do
  mkdir -p "charts/$s/templates"
  cp _service-templates/_helpers.tpl \
     _service-templates/deployment.yaml \
     _service-templates/service.yaml \
     _service-templates/pdb.yaml \
     "charts/$s/templates/"
  echo "stamped charts/$s/templates"
done
