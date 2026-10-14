# Chart and CI patterns

Proven snippets from the Kithly migration (GitHub Actions + GHCR + SSH deploy). Adapt names;
keep the mechanics. Lesson IDs refer to `../references/lessons.md`.

## Chart layout

```
deploy/helm/<app>/
  Chart.yaml
  values.yaml          # services: map of name → {binary, port, replicas, probe, resources, expose}
  templates/
    _helpers.tpl       # labels (define app.kubernetes.io/instance ONCE — L26 caught a duplicate)
    services.yaml      # range over .Values.services → Deployment + Service
    gateway.yaml       # Gateway + HTTPRoutes (L15)
    redis.yaml         # if Compose had one: Deployment/StatefulSet + PVC, AOF
    postgres.yaml      # if the DB moves: StatefulSet, secret via lookup, backup CronJob
```

## Roll pods when secrets change (L21)

```yaml
# templates/services.yaml, pod template
metadata:
  annotations:
    <app>/env-checksum: {{ $.Values.envChecksum | quote }}
spec:
  imagePullSecrets: [{name: ghcr-pull}]
  containers:
    - envFrom: [{secretRef: {name: <app>-env}}]
```

## Generated, upgrade-stable password (L22)

```yaml
{{- $name := printf "%s-postgres" .Release.Name }}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $name }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $name }}
  annotations:
    helm.sh/resource-policy: keep
data:
  POSTGRES_PASSWORD: {{ if $existing }}{{ index $existing.data "POSTGRES_PASSWORD" }}{{ else }}{{ randAlphaNum 32 | b64enc }}{{ end }}
```

## Backups to a hostPath, not a PVC (L23)

```yaml
# CronJob pod spec
volumes:
  - name: backups
    hostPath: {path: {{ .Values.postgres.backup.hostPath }}, type: DirectoryOrCreate}
```

## Traefik Gateway API provider (L15) — written by bootstrap.sh

`<data-dir>/server/manifests/traefik-config.yaml`: `HelmChartConfig` enabling
`providers.kubernetesGateway`, default `gateway` off, `gatewayClass` on. The chart's `Gateway`
uses `gatewayClassName: traefik` and an HTTP listener on port 8000 — the container port of
Traefik's `web` entrypoint, which host :80 maps to.

## CI: PR validation (L26)

```yaml
- name: Lint and validate rendered manifests
  env:
    CRD_SCHEMAS: https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json
  run: |
    kubeconform() { command kubeconform -schema-location default -schema-location "$CRD_SCHEMAS" "$@"; }
    helm lint "$CHART_DIR" --set image.tag=ci
    # one render per feature-flag variant
    helm template "$RELEASE" "$CHART_DIR" --set image.tag=ci | kubeconform -strict -summary -kubernetes-version "$K8S_VERSION"
    helm template "$RELEASE" "$CHART_DIR" --set image.tag=ci --set database.inCluster=true | kubeconform -strict -summary -kubernetes-version "$K8S_VERSION"
```

## CI: build, one image per binary (L18)

```yaml
- uses: docker/build-push-action@v6
  with:
    build-args: BINARY=${{ matrix.binary }}
    push: true
    provenance: false   # one package version per push, so count-based cleanup can't split an image
    tags: ghcr.io/<owner>/<app>-${{ matrix.binary }}:${{ github.sha }}
    cache-from: type=gha,scope=${{ matrix.binary }}
    cache-to: type=gha,mode=max,scope=${{ matrix.binary }}
```

Cleanup after a successful deploy: `actions/delete-package-versions@v5` with
`package-type: container`, `min-versions-to-keep: 20`.

## CI: secret sync over SSH stdin (L20, L21)

```bash
umask 077
for k in $required; do [ -n "${!k}" ] || { echo "::error::secret $k is not set"; exit 1; }; done
: > app.env
for k in $all; do [ -n "${!k}" ] && printf '%s=%s\n' "$k" "${!k}" >> app.env; done
echo "ENV_CHECKSUM=$(sha256sum app.env | cut -c1-16)" >> "$GITHUB_ENV"

apply_secret() {   # $1 name, $2 kubectl args; file on stdin
  ssh "$DEPLOY_HOST" "set -e; f=\$(mktemp); trap 'rm -f \$f' EXIT; cat > \$f; \
    kubectl -n $NAMESPACE create secret generic $1 $2=\$f --dry-run=client -o yaml | kubectl apply -f -"
}
apply_secret <app>-env --from-env-file < app.env
apply_secret ghcr-pull "--type=kubernetes.io/dockerconfigjson --from-file=.dockerconfigjson" < dockerconfig.json
rm -f app.env dockerconfig.json
```

## CI: deploy (L25, L28)

```bash
# pre-flight
ssh "$DEPLOY_HOST" 'command -v helm >/dev/null && kubectl get ns "'"$NAMESPACE"'" >/dev/null' \
  || { echo "::error::cluster not bootstrapped — run the bootstrap workflow first"; exit 1; }
# chart travels as a tarball; no repo checkout on the server
tar -C "$(dirname "$CHART_DIR")" -cz "$(basename "$CHART_DIR")" \
  | ssh "$DEPLOY_HOST" 'rm -rf ~/chart && mkdir -p ~/chart && tar -xz -C ~/chart'
ssh "$DEPLOY_HOST" "helm upgrade --install $RELEASE ~/chart/$(basename "$CHART_DIR") -n $NAMESPACE \
  --set image.tag=$GITHUB_SHA --set-string envChecksum=$ENV_CHECKSUM \
  --rollback-on-failure --timeout 10m"
```

Always: `helm history --max 5` and `kubectl get deploy,pods,svc`. On failure: last 40 events +
`kubectl logs --all-containers --tail=30` per pod.

Bootstrap workflow: manual `workflow_dispatch` with a typed confirmation input
(`confirm: <hostname>`), same concurrency group as the Compose deploy workflow during overlap (L13).

## Pre-cutover checks (L12)

```bash
# Through Traefik's NodePort, with the public Host header
port=$(kubectl -n kube-system get svc traefik -o jsonpath='{.spec.ports[?(@.name=="web")].nodePort}')
curl -fsS -H 'Host: api.example.com' "http://$NODE_IP:$port/health"
```
The hostPort-on-localhost check is built into `bootstrap.sh`.
