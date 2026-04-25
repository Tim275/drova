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
        sync('./frontend', '/app/frontend'),
    ],
)

docker_compose('./docker-compose.yaml')
