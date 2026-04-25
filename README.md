# Drova – Ride Sharing

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose
- [Go 1.24+](https://go.dev/dl/) (only for local development without Docker)
- A free [Mapbox](https://account.mapbox.com/) account for the API token

## Setup

**1. Clone the repo**
```bash
git clone https://github.com/Tim275/drova.git
cd drova
```

**2. Create `.env` file in the project root**
```env
MAPBOX_TOKEN=pk.your_secret_token_here
MAPBOX_PUBLIC_TOKEN=pk.your_public_token_here
```

> Get your tokens at https://account.mapbox.com → Tokens

**3. Start all services**
```bash
docker compose up --build
```

**4. Open the app**

→ http://localhost:8081

## Services

| Service | Port | Description |
|---|---|---|
| api-gateway | 8081 | HTTP + WebSocket, serves frontend |
| trip-service | 9093 | gRPC, route calculation + fare estimation |
| driver-service | 9092 | gRPC, driver registration |

## Usage

**As a Rider:**
1. Click *I Need a Ride*
2. Enter pickup and destination address
3. Click *Calculate Route*
4. Select a vehicle category and confirm

**As a Driver:**
1. Click *I Want to Drive*
2. Enter your package (economy / comfort / van / business)
3. Wait for a ride request

## Architecture

```
Browser
  └── HTTP/WS → api-gateway :8081
                    ├── gRPC → trip-service :9093
                    └── gRPC → driver-service :9092
```

## Not done yet

- Payment happens before the driver accepts. Should be pre-auth on booking, capture once accepted.
- Kubernetes deployment — planned with Istio and Cilium NetworkPolicies.
- Persistent storage — trips and drivers still in-memory.
