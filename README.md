# Drova

Production-grade microservices backend in Go — event-driven architecture with Kafka, gRPC, and GitOps deployment to Kubernetes.

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| API | REST (api-gateway) + gRPC (internal) |
| Messaging | Kafka 3.7 (KRaft, 15 topics) |
| Databases | PostgreSQL · Redis · MongoDB |
| Payments | Stripe (Checkout Sessions + Webhook) |
| Maps | Mapbox GL JS + Geocoding + Directions API |
| Observability | Jaeger (OTLP tracing) + zap (structured logging) |
| Schema | Confluent Schema Registry + Avro |
| Security | JWT + JTI + Redis Blacklist + bcrypt + SASL/PLAIN |

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- [Stripe CLI](https://stripe.com/docs/stripe-cli) (for webhook forwarding in dev)
- A [Mapbox](https://account.mapbox.com/) account (free tier works)
- A [Stripe](https://dashboard.stripe.com/) account (test mode)

---

## Test Accounts

Pre-seeded accounts — no email verification needed:

| Role | Email | Password |
|---|---|---|
| Rider | `rider@drova.local` | `Test1234!` |
| Driver | `driver@drova.local` | `Test1234!` |

---

## Local Dev (Tilt)

```bash
tilt up    # live-reload without Docker rebuild
tilt down
```
