load('ext://restart_process', 'docker_build_with_restart')

# --- Trip Service ---
local_resource(
    'trip-service-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/trip-service ./services/trip-service/cmd',
    deps=['./services/trip-service', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-trip-service',
    context='.',
    dockerfile='services/trip-service/Dockerfile.tilt',
    entrypoint='/app/trip-service',
    live_update=[
        sync('./build/trip-service', '/app/trip-service'),
    ],
)

# --- Driver Service ---
local_resource(
    'driver-service-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/driver-service ./services/driver-service',
    deps=['./services/driver-service', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-driver-service',
    context='.',
    dockerfile='services/driver-service/Dockerfile.tilt',
    entrypoint='/app/driver-service',
    live_update=[
        sync('./build/driver-service', '/app/driver-service'),
    ],
)

# --- API Gateway ---
local_resource(
    'api-gateway-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/api-gateway ./services/api-gateway',
    deps=['./services/api-gateway', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-api-gateway',
    context='.',
    dockerfile='services/api-gateway/Dockerfile.tilt',
    entrypoint='/app/api-gateway',
    live_update=[
        sync('./build/api-gateway', '/app/api-gateway'),
    ],
)

# --- Frontend (nginx serves index.html) ---
docker_build(
    'drova-frontend',
    context='./frontend',
    dockerfile='./frontend/Dockerfile',
    live_update=[
        sync('./frontend/index.html', '/usr/share/nginx/html/index.html'),
    ],
)

# --- User Service ---
local_resource(
    'user-service-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/user-service ./services/user-service/cmd',
    deps=['./services/user-service', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-user-service',
    context='.',
    dockerfile='services/user-service/Dockerfile.tilt',
    entrypoint='/app/user-service',
    live_update=[
        sync('./build/user-service', '/app/user-service'),
        sync('./services/user-service/migrations', '/app/migrations'),
    ],
)

# --- Payment Service ---
local_resource(
    'payment-service-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/payment-service ./services/payment-service/cmd',
    deps=['./services/payment-service', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-payment-service',
    context='.',
    dockerfile='services/payment-service/Dockerfile.tilt',
    entrypoint='/app/payment-service',
    live_update=[
        sync('./build/payment-service', '/app/payment-service'),
        sync('./services/payment-service/migrations', '/app/migrations'),
    ],
)

# --- Chat Service ---
local_resource(
    'chat-service-compile',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/chat-service ./services/chat-service',
    deps=['./services/chat-service', './shared'],
    dir='.',
)

docker_build_with_restart(
    'drova-chat-service',
    context='.',
    dockerfile='services/chat-service/Dockerfile.tilt',
    entrypoint='/app/chat-service',
    live_update=[
        sync('./build/chat-service', '/app/chat-service'),
    ],
)

docker_compose('./docker-compose.yaml')
