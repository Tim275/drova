# Drova — Homelab Deployment

## Homelab-Architektur

```
┌─────────────────────────────────────────────────────────────────────┐
│  HOMELAB (pi-cluster)                                               │
│  Raspberry Pi · Talos Linux · Flux CD (GitOps)                      │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Shared Infrastructure (vom Homelab verwaltet)                │  │
│  │  ├── Envoy Gateway    (Gateway API — HTTPRoute)               │  │
│  │  ├── Cert-Manager     (Let's Encrypt TLS)                     │  │
│  │  ├── CloudNativePG    (PostgreSQL Cluster)                    │  │
│  │  ├── MongoDB Operator (Community Edition)                     │  │
│  │  ├── Minio            (S3 für CNPG-Backups)                   │  │
│  │  ├── Redis            (Cache + Blacklist)                     │  │
│  │  ├── Kafka            (Event Broker)                          │  │
│  │  ├── Jaeger / Tempo   (Distributed Tracing)                   │  │
│  │  ├── Prometheus+Grafana (Metriken)                            │  │
│  │  ├── Longhorn         (Block Storage)                         │  │
│  │  └── Sealed Secrets   (Secrets in Git verschlüsseln)          │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Namespace: drova   ← infra/production/k8s/                  │  │
│  │                                                               │  │
│  │  ┌────────────┐  ┌──────────────┐  ┌──────────────┐          │  │
│  │  │ api-gateway│  │ user-service │  │ trip-service │          │  │
│  │  │   :8081    │  │    :8082     │  │  :9093/:8083 │          │  │
│  │  └────────────┘  └──────────────┘  └──────────────┘          │  │
│  │  ┌──────────────┐  ┌───────────────┐  ┌────────────────┐     │  │
│  │  │driver-service│  │payment-service│  │  chat-service  │     │  │
│  │  │    :9092     │  │  (kafka-only) │  │     :8084      │     │  │
│  │  └──────────────┘  └───────────────┘  └────────────────┘     │  │
│  │                                                               │  │
│  │  HTTPRoute (Envoy Gateway API)                                │  │
│  │  drova.timourhomelab.org/ws/chat  → chat-service:8084        │  │
│  │  drova.timourhomelab.org/*        → api-gateway:8081         │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Infra-Verzeichnisse

```
infra/
├── development/k8s/      → Lokale K8s-Tests (kind/minikube)
└── production/k8s/       → Homelab Deployment (nur App-Schicht)
    ├── kustomization.yaml
    ├── namespace.yaml          Namespace drova + ResourceQuota + LimitRange
    ├── app-config.yaml         ConfigMap (Env-Vars, Service-Discovery, Homelab-Endpoints)
    ├── sealed-secrets.template.yaml  Secret-Vorlagen (nie committen!)
    ├── services/
    │   ├── api-gateway.yaml
    │   ├── user-service.yaml
    │   ├── trip-service.yaml
    │   ├── driver-service.yaml
    │   ├── payment-service.yaml
    │   └── chat-service.yaml
    └── ingress/
        └── httproute.yaml      Envoy Gateway API HTTPRoute
```

Datenbanken, Kafka, Redis, Jaeger — alles wird vom Homelab provisioniert.
Drova deployt nur seine eigenen Pods + HTTPRoute.

## Gateway API (Envoy)

```bash
# Verfügbare Gateways anzeigen
kubectl get gateway -A

# parentRefs in ingress/httproute.yaml anpassen:
#   name: <gateway-name>
#   namespace: <gateway-namespace>
```

WebSocket-Upgrade (WS/WSS) wird von Envoy Gateway automatisch durchgeleitet — kein spezielles Annotation nötig.

Die Chat-Route muss vor dem catch-all `/` stehen (längerer Pfad gewinnt).

## Secrets-Management (Sealed Secrets)

```bash
# 1. Template befüllen
cp infra/production/k8s/sealed-secrets.template.yaml /tmp/drova-secret.yaml
# → Werte eintragen

# 2. Versiegeln
kubeseal --format=yaml \
  --controller-namespace=sealed-secrets \
  --controller-name=sealed-secrets \
  < /tmp/drova-secret.yaml \
  > infra/production/k8s/drova-secrets.sealed.yaml

# 3. Nur das .sealed.yaml committen
```

Benötigte Secrets:
- `drova-app-secrets` — JWT_SECRET, Mapbox, Stripe, SMTP
- `drova-pg-{users,trips,drivers,payments}-creds` — Postgres App-User
- `drova-redis-secret` — Redis Passwort

## Container Images bauen und pushen

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u Tim275 --password-stdin

for svc in api-gateway user-service trip-service driver-service payment-service chat-service; do
  docker build -f services/$svc/Dockerfile -t ghcr.io/Tim275/drova/$svc:latest .
  docker push ghcr.io/Tim275/drova/$svc:latest
done
```

## Deployment via Flux CD

```
pi-cluster/
└── apps/
    ├── base/
    │   └── drova/         ← infra/production/k8s/ Inhalt hier
    └── staging/
        └── drova/
            └── kustomization.yaml
```

```yaml
# apps/staging/drova/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base/drova
```

Flux registrieren (einmalig):
```bash
kubectl apply -f - <<EOF
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: drova
  namespace: flux-system
spec:
  interval: 10m
  path: ./apps/staging/drova
  prune: true
  sourceRef:
    kind: GitRepository
    name: pi-cluster
  dependsOn:
    - name: infrastructure-controllers
EOF
```

## initContainer-Migrations

Für Services mit Postgres-Migrations (user, trip, driver, payment) läuft ein initContainer:

```yaml
initContainers:
  - name: migrate
    image: ghcr.io/Tim275/drova/<service>:latest
    command: ["/app/<service>", "migrate"]
    env:
      - name: DB_URL
        valueFrom:
          secretKeyRef:
            name: drova-pg-<service>-creds
            key: DB_URL
```

Migrations laufen vor dem Service-Start — schlägt die Migration fehl, startet der Pod nicht.

## Domains

| Domain | Ziel |
|--------|------|
| `drova.timourhomelab.org` | api-gateway (App + REST + WebSocket) |
| `drova.timourhomelab.org/ws/chat` | chat-service (WebSocket) |
