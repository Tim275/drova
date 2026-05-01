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

[docs/architecture.excalidraw](docs/architecture.excalidraw) — open in [Excalidraw](https://excalidraw.com)

---

## Run Locally

```bash
tilt up     # starts everything with live reload
tilt down
```

Requires Docker, a Mapbox token, and a Stripe test account. Copy `.env.example` to `.env` and fill in the values.
