#!/usr/bin/env bash
# Installs single-node k3s and Helm next to a live Docker Compose stack, without
# touching its ports (ServiceLB stays off until cutover — lessons L10, L12).
# Run over SSH as the deploy user (needs passwordless sudo):
#   ssh <host> 'NODE_IP=… UPSTREAM_DNS=… NAMESPACE=… bash -s' < bootstrap.sh
#
# Template from the compose-to-k3s skill: adapt the defaults to the host's
# HOST_FACTS (disks, NICs, DNS, firewall), and drop sections that don't apply.
#
# Idempotent: re-running converges the host to this config and only restarts
# k3s when its config or version actually changed. The Docker Compose stacks
# already on the host are left alone — k3s runs its own containerd.
set -euo pipefail

K3S_VERSION="${K3S_VERSION:-v1.36.5+k3s1}"
HELM_VERSION="${HELM_VERSION:-v4.1.3}"
# Wired ethernet. The host also has wifi on the same subnet; pin so the node
# never advertises the wifi address.
NODE_IP="${NODE_IP:?set NODE_IP (the wired NIC address)}"
NAMESPACE="${NAMESPACE:?set NAMESPACE}"
# Plain IPv4 upstream(s), space-separated. Host resolv.conf files with IPv6
# link-local servers (fe80::…%2) or >3 entries break CoreDNS (L4), so k3s gets
# a clean file of its own.
UPSTREAM_DNS="${UPSTREAM_DNS:?set UPSTREAM_DNS (e.g. the LAN router)}"
LB_LABEL=svccontroller.k3s.cattle.io/enablelb
LB_DROPIN=/etc/rancher/k3s/config.yaml.d/50-servicelb.yaml

# Put images/volumes (DATA_DIR) and pod scratch (KUBELET_DIR) on the disk with
# room (L3). Leave KUBELET_DIR empty to keep /var/lib/kubelet where it is.
DATA_DIR="${DATA_DIR:-/var/lib/rancher/k3s}"
KUBELET_DIR="${KUBELET_DIR:-}"

log() { printf '\n==> %s\n' "$*"; }

if [ -n "$KUBELET_DIR" ]; then
  log "Kubelet pod storage on $KUBELET_DIR (bind mount over /var/lib/kubelet)"
  sudo mkdir -p "$KUBELET_DIR" /var/lib/kubelet
  if ! grep -qE '[[:space:]]/var/lib/kubelet[[:space:]]' /etc/fstab; then
    echo "$KUBELET_DIR /var/lib/kubelet none bind 0 0" | sudo tee -a /etc/fstab >/dev/null
  fi
  mountpoint -q /var/lib/kubelet || sudo mount /var/lib/kubelet
fi

log "Kernel networking prerequisites"
printf 'overlay\nbr_netfilter\n' | sudo tee /etc/modules-load.d/k3s.conf >/dev/null
sudo modprobe overlay
sudo modprobe br_netfilter
printf 'net.ipv4.ip_forward = 1\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\n' \
  | sudo tee /etc/sysctl.d/90-k3s.conf >/dev/null
sudo sysctl --quiet --system

if command -v ufw >/dev/null && sudo ufw status | grep -q 'Status: active'; then
  log "ufw: allow pod (10.42/16) and service (10.43/16) traffic"
  # Without these, ufw drops pod -> CoreDNS / ClusterIP traffic and DNS fails silently (L5).
  sudo ufw allow from 10.42.0.0/16 to any comment 'k3s pods' >/dev/null
  sudo ufw allow from 10.43.0.0/16 to any comment 'k3s services' >/dev/null
fi

log "k3s config"
sudo mkdir -p /etc/rancher/k3s/config.yaml.d "$DATA_DIR/server/manifests"

# ServiceLB switch. ServiceLB's hostPort rules are iptables DNAT, not a socket
# bind, so they do NOT wait for Compose to release :80 — they hijack it the
# moment they exist. It must therefore be off from the very first boot, which
# only a registration-time node label achieves. k3s re-applies node-label on
# every start, so this file is the persistent half of the switch; created once,
# then owned by deploy/k3s/servicelb.sh (cutover/rollback). Never rewritten here.
if [ ! -f "$LB_DROPIN" ]; then
  printf 'node-label:\n  - %s=false\n' "$LB_LABEL" | sudo tee "$LB_DROPIN" >/dev/null
fi
echo "ServiceLB drop-in: $(sudo cat "$LB_DROPIN" | tr -d '\n')"
printf 'nameserver %s\n' $UPSTREAM_DNS | sudo tee /etc/rancher/k3s/resolv.conf >/dev/null

# Bundled Traefik with the Gateway API provider on. Its LoadBalancer Service is
# bound to host :80/:443 by ServiceLB once that is switched on at cutover.
# The default Gateway is off: the app chart defines its own, next to its routes.
# Picked up automatically by k3s from the manifests dir.
sudo tee "$DATA_DIR/server/manifests/traefik-config.yaml" >/dev/null <<'EOF'
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: traefik
  namespace: kube-system
spec:
  valuesContent: |-
    providers:
      kubernetesGateway:
        enabled: true
    gateway:
      enabled: false
    gatewayClass:
      enabled: true
EOF

config=$(cat <<EOF
data-dir: $DATA_DIR
node-ip: $NODE_IP
resolv-conf: /etc/rancher/k3s/resolv.conf
# Readable by the deploy user's group: k3s's bundled kubectl always reads this
# file (it ignores ~/.kube/config unless KUBECONFIG is set).
write-kubeconfig-mode: "0640"
write-kubeconfig-group: $(id -gn)
# The host keeps swap for the Compose workloads; pods themselves do not swap.
kubelet-arg:
  - fail-swap-on=false
EOF
)
config_changed=false
if [ "$(sudo cat /etc/rancher/k3s/config.yaml 2>/dev/null || true)" != "$config" ]; then
  printf '%s\n' "$config" | sudo tee /etc/rancher/k3s/config.yaml >/dev/null
  config_changed=true
fi

installed=$(k3s --version 2>/dev/null | awk 'NR==1 {print $3}' || true)
if [ "$installed" != "$K3S_VERSION" ]; then
  log "Installing k3s $K3S_VERSION (currently: ${installed:-none})"
  # Installer pinned to the same tag as the binary rather than get.k3s.io (moving target).
  curl -sfL "https://raw.githubusercontent.com/k3s-io/k3s/${K3S_VERSION/+/%2B}/install.sh" \
    | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - server
elif $config_changed; then
  log "k3s config changed, restarting"
  sudo systemctl restart k3s
else
  log "k3s $K3S_VERSION already installed, config unchanged"
fi

log "Waiting for node Ready"
for _ in $(seq 1 60); do
  sudo k3s kubectl get nodes >/dev/null 2>&1 && break
  sleep 3
done
sudo k3s kubectl wait --for=condition=Ready node --all --timeout=180s

log "Kubeconfig for $(whoami)"
mkdir -p "$HOME/.kube"
sudo install -m 600 -o "$(id -u)" -g "$(id -g)" /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"

if [ "$(helm version --template '{{.Version}}' 2>/dev/null || true)" != "$HELM_VERSION" ]; then
  log "Installing Helm $HELM_VERSION"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  tarball="helm-${HELM_VERSION}-linux-amd64.tar.gz"
  curl -sfL -o "$tmp/$tarball" "https://get.helm.sh/$tarball"
  curl -sfL -o "$tmp/$tarball.sha256sum" "https://get.helm.sh/$tarball.sha256sum"
  (cd "$tmp" && sha256sum -c "$tarball.sha256sum")
  tar -xzf "$tmp/$tarball" -C "$tmp"
  sudo install -m 755 "$tmp/linux-amd64/helm" /usr/local/bin/helm
  rm -rf "$tmp"
  trap - EXIT
fi

log "Traefik + Gateway API"
# The Gateway API CRDs come from k3s's own traefik-crd chart (v1.6.1 in k3s
# v1.36.5) — don't install a second copy, the two would fight over ownership.
for _ in $(seq 1 100); do
  kubectl -n kube-system get deployment/traefik >/dev/null 2>&1 && break
  sleep 3
done
kubectl -n kube-system get deployment/traefik >/dev/null   # surfaces the real error if the loop timed out
kubectl get crd gateways.gateway.networking.k8s.io httproutes.gateway.networking.k8s.io >/dev/null
kubectl -n kube-system rollout status deployment/traefik --timeout=300s
for _ in $(seq 1 40); do
  kubectl get gatewayclass traefik >/dev/null 2>&1 && break
  sleep 3
done
kubectl wait --for=condition=Accepted gatewayclass/traefik --timeout=120s

log "ServiceLB state"
node=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "ServiceLB on $node: $(kubectl get node "$node" -o jsonpath="{.metadata.labels.${LB_LABEL//./\\.}}")"

log "Check hostPorts are reachable on localhost (what cloudflared uses)"
# ServiceLB exposes ports as plain pod hostPorts, so a throwaway hostPort pod on a
# free port proves the exact path cloudflared will take (http://localhost:<port>)
# — without switching ServiceLB on and touching :80.
kubectl -n default delete pod hostport-check --ignore-not-found --wait >/dev/null
kubectl -n default apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: hostport-check
spec:
  containers:
    - name: whoami
      image: traefik/whoami:v1.11.0
      ports:
        - containerPort: 80
          hostPort: 18080
EOF
trap 'kubectl -n default delete pod hostport-check --ignore-not-found >/dev/null' EXIT
kubectl -n default wait --for=condition=Ready pod/hostport-check --timeout=120s
ok=false
for _ in $(seq 1 15); do
  if curl -fsS -o /dev/null http://localhost:18080/; then ok=true; break; fi
  sleep 2
done
kubectl -n default delete pod hostport-check --ignore-not-found >/dev/null
trap - EXIT
$ok || { echo "!! hostPort not reachable via localhost — cloudflared would not reach k8s"; exit 1; }
echo "localhost -> hostPort OK"

log "Namespace $NAMESPACE"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

log "Done"
k3s --version | head -1
helm version --short
kubectl get nodes -o wide
df -h / "$DATA_DIR"
