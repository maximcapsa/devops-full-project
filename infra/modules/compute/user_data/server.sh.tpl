#!/bin/bash
# k3s server bootstrap (AL2023 arm64). Publishes the join token and a
# kubeconfig (rewritten to the Elastic IP) to SSM Parameter Store.
set -euxo pipefail

# tls-san so kubectl/helm can talk to the API via the public EIP.
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --tls-san ${eip}" sh -

# Wait for k3s to write the node token, then publish it for the agents.
while [ ! -f /var/lib/rancher/k3s/server/node-token ]; do sleep 2; done
aws ssm put-parameter \
  --name "${token_param}" \
  --value "$(cat /var/lib/rancher/k3s/server/node-token)" \
  --type SecureString --overwrite --region "${region}"

# Publish kubeconfig pointed at the EIP (Advanced tier: file exceeds 4KB).
sed "s/127.0.0.1/${eip}/" /etc/rancher/k3s/k3s.yaml > /tmp/kubeconfig
aws ssm put-parameter \
  --name "${kubeconfig_param}" \
  --value "file:///tmp/kubeconfig" \
  --type SecureString --tier Advanced --overwrite --region "${region}"
rm -f /tmp/kubeconfig
