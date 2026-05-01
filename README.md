# Drova

Ridesharing backend built as Go microservices — Kafka, gRPC, Stripe, Mapbox, deployed on Kubernetes with GitOps.

**Live:** https://drova.timourhomelab.org

---

## Test Accounts

| Role   | Email                  | Password    |
|--------|------------------------|-------------|
| Rider  | `rider@drova.local`   | `Test1234!` |
| Driver | `driver@drova.local`  | `Test1234!` |

No email verification needed for these accounts.

---

## Architecture

[![Architecture Diagram](docs/architecture.excalidraw)](docs/architecture.excalidraw)

> Open `docs/architecture.excalidraw` in [Excalidraw](https://excalidraw.com) for the full diagram.

---

## Stack

| | |
|---|---|
| Language | Go 1.25 |
| Services | api-gateway · user-service · trip-service · driver-service · payment-service · chat-service |
| Messaging | Kafka (KRaft, 14 topics) |
| Databases | PostgreSQL · Redis · MongoDB |
| Internal RPC | gRPC |
| Payments | Stripe Checkout + Webhook |
| Maps | Mapbox GL JS + Directions API |
| Observability | Jaeger (OTLP) + zap |
| Deploy | ArgoCD + CNPG + Kubernetes |

---

## Run Locally

```bash
tilt up     # starts everything with live reload
tilt down
```

Requires Docker, a Mapbox token, and a Stripe test account. Copy `.env.example` to `.env` and fill in the values.
