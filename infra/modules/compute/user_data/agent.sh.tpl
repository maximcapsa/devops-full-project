#!/bin/bash
# k3s agent bootstrap (AL2023 arm64, spot). Polls SSM until the server has
# published the join token — agents may boot before the server finishes.
set -euxo pipefail

until TOKEN=$(aws ssm get-parameter \
    --name "${token_param}" \
    --with-decryption \
    --query Parameter.Value \
    --output text \
    --region "${region}" 2>/dev/null); do
  echo "waiting for k3s token in SSM..."
  sleep 10
done

curl -sfL https://get.k3s.io | K3S_URL="https://${server_ip}:6443" K3S_TOKEN="$TOKEN" sh -
