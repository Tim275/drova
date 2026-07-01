# Local-only, NATIVE-ARCH build for the KEDA demo.
#
# Drova's production Dockerfile (services/trip-service/Dockerfile) pins the base
# images by amd64 digests. On Apple Silicon (colima = aarch64) that forces the
# builder to run under QEMU x86_64 emulation, where `go mod download` crashes
# (runtime/asm_amd64.s ... makeslice/mallocgc). Using plain tags lets docker pull
# the host-native arm64 variants, so the build AND the runtime pod run natively.
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
    -o /out/trip-service ./services/trip-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/trip-service /app/trip-service
COPY --from=builder /app/services/trip-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 9093
ENTRYPOINT ["/app/trip-service"]
