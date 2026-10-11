#!/usr/bin/env bash
# Switches ServiceLB — and with it k8s ownership of host :80/:443/:3002/:3003 —
# on or off. This is the Compose -> k8s cutover switch and its rollback.
#
#   ssh <user>@<host> 'bash -s' -- on  < deploy/k3s/servicelb.sh
#   ssh <user>@<host> 'bash -s' -- off < deploy/k3s/servicelb.sh
#
# Both halves are needed: the node label takes effect immediately, and the k3s
# config drop-in keeps it that way, because k3s re-applies node-label on every
# start (a reboot would otherwise revert the label).
#
# "on" while Compose still listens on :80/:3002 hijacks that traffic — stop
# Compose first.
set -euo pipefail

LB_LABEL=svccontroller.k3s.cattle.io/enablelb
LB_DROPIN=/etc/rancher/k3s/config.yaml.d/50-servicelb.yaml

case "${1:-}" in
  on) value=true ;;
  off) value=false ;;
  *) echo "usage: servicelb.sh on|off" >&2; exit 2 ;;
esac

if [ "$value" = true ] && sudo ss -ltnpH '( sport = :80 or sport = :3002 )' | grep -q docker; then
  echo "!! Docker still listens on :80/:3002 — stop those containers first" >&2
  exit 1
fi

printf 'node-label:\n  - %s=%s\n' "$LB_LABEL" "$value" | sudo tee "$LB_DROPIN" >/dev/null
node=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
kubectl label node "$node" "$LB_LABEL=$value" --overwrite

if [ "$value" = true ]; then
  echo "waiting for ServiceLB pods..."
  sleep 5
  kubectl -n kube-system wait --for=condition=Ready pod -l svccontroller.k3s.cattle.io/svcnamespace --timeout=120s
else
  for _ in $(seq 1 30); do
    [ -z "$(kubectl -n kube-system get pods -l svccontroller.k3s.cattle.io/svcnamespace -o name)" ] && break
    sleep 2
  done
fi
kubectl -n kube-system get pods -l svccontroller.k3s.cattle.io/svcnamespace -o wide
echo "ServiceLB: $1"
