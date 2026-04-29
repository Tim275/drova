# Drova — Ride Sharing Platform

Microservices-based ride sharing backend (Go) — similar to Uber/Bolt.
Built as a learning project + portfolio + consulting reference.

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

## Setup

**1. Clone**
```bash
git clone https://github.com/Tim275/drova.git
cd drova
```

**2. Create `.env` in the project root**
```env
# Mapbox
MAPBOX_TOKEN=pk.your_secret_token           # Backend (URL-restricted)
MAPBOX_PUBLIC_TOKEN=pk.your_public_token    # Frontend map display

# PostgreSQL / Redis / MongoDB
POSTGRES_PASSWORD=your_strong_password
REDIS_PASSWORD=your_redis_password
MONGO_USERNAME=admin
MONGO_PASSWORD=your_mongo_password
MONGO_URI=mongodb://admin:your_mongo_password@mongodb:27017/drova?authSource=admin

# JWT
JWT_SECRET=supersecretjwtsecret32byteslong!!   # min 32 chars

# App
APP_BASE_URL=http://localhost:8081

# Stripe
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLIC_KEY=pk_test_...
STRIPE_CURRENCY=eur
STRIPE_SUCCESS_URL=http://localhost:8081/?payment=success
STRIPE_CANCEL_URL=http://localhost:8081/?payment=cancel
STRIPE_WEBHOOK_KEY=whsec_...    # from: stripe listen --forward-to ...

# Email (Gmail SMTP with App Password)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your@gmail.com
SMTP_PASSWORD=your_app_password
FROM_EMAIL=your@gmail.com
```

**3. Start everything**
```bash
docker compose up --build
```

**4. Forward Stripe webhooks** (separate terminal)
```bash
stripe listen --forward-to localhost:8081/webhook/stripe
# Copy the whsec_... key into .env → STRIPE_WEBHOOK_KEY, then restart api-gateway
```

**5. Open the app**

→ http://localhost:8081

---

## Test Accounts

Pre-seeded accounts — no email verification needed:

| Role | Email | Password |
|---|---|---|
| Rider | `rider@drova.local` | `Test1234!` |
| Driver | `driver@drova.local` | `Test1234!` |

---

## Services & Ports

| Service | Port | Description |
|---|---|---|
| **api-gateway** | 8081 | HTTP + WebSocket gateway, serves frontend |
| **user-service** | 8082 | Auth (JWT + JTI), registration, profile |
| **trip-service** | 9093 (gRPC) / 8083 (HTTP) | Route, fare, trip lifecycle |
| **driver-service** | 9092 (gRPC) | Driver registration, location streaming |
| **payment-service** | — (Kafka only) | Stripe Checkout, payment records |
| **chat-service** | 8084 | WebSocket chat, Redis Pub/Sub, MongoDB |
| **Kafka** | 9092 | 15 topics, KRaft mode |
| **Kafka UI** | 8080 | Redpanda Console |
| **Schema Registry** | 8085 | Confluent Schema Registry (Avro) |
| **Jaeger** | 16686 | Distributed tracing UI |
| **PostgreSQL** | 5432 | Users, trips, payments |
| **Redis** | 6379 | JWT blacklist, rate limiting, chat Pub/Sub |
| **MongoDB** | 27017 | Chat messages |

---

## Trip Flow

```
Rider → POST /trip/start
  → trip-service creates trip (status: searching)
  → Kafka: trip.event.created

driver-service picks available driver
  → Kafka: driver.cmd.trip_request → Driver WS popup

Driver accepts → Kafka: driver.cmd.trip_response
  → trip-service: status = accepted
  → Kafka: trip.event.driver_assigned → Rider WS

Driver: arrived → trip_start → trip_end
  → Kafka: trip.event.completed
  → payment-service: Stripe Checkout Session
  → Rider: payment link via WS

Rider pays → Stripe Webhook → Kafka: payment.event.success
Rider rates (1–5 ★) → POST /trip/rate
```

---

## Architecture

```
Browser (Rider/Driver)
  └── HTTP/WebSocket → api-gateway :8081
                          ├── gRPC → trip-service :9093
                          ├── gRPC → driver-service :9092
                          └── Kafka (15 topics)
                                ├── trip-service (consumer + publisher)
                                ├── driver-service (consumer + publisher)
                                └── payment-service (consumer + publisher)
```

---

## Enterprise Features

| Feature | Implementation |
|---|---|
| JWT Blacklist | Redis (`jti:revoked:{jti}`) checked in api-gateway + user-service |
| Rate Limiting | Redis Sliding Window (user-service) + Token Bucket (api-gateway) |
| Circuit Breaker | sony/gobreaker on all gRPC calls (3 failures → open → 503) |
| Tracing | Jaeger OTLP across all 6 services |
| Schema Registry | Confluent-compatible, Avro wire format (`shared/schema`) |
| Kafka SASL | SASL/PLAIN via `KAFKA_SASL_USERNAME` / `KAFKA_SASL_PASSWORD` |
| CORS | Configurable via `CORS_ALLOWED_ORIGIN` env var |
| Security Headers | X-Content-Type-Options, X-Frame-Options, CSP, X-Request-ID |
| DLQ | Dead Letter Queue for failed Kafka messages |
| Graceful Shutdown | SIGTERM + 10s timeout in all services |
| mTLS | Via Istio (infrastructure layer, not application code) |

---

## Local Dev (Tilt)

```bash
tilt up    # live-reload without Docker rebuild
tilt down
```
