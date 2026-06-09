FROM golang:1.26-alpine@sha256:103c743516b0d9dd69c203ed64f730eb342cae4b85d3f6c5cb376d91abbc6bcb AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/api-gateway ./services/api-gateway

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/api-gateway /app/api-gateway
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/app/api-gateway"]
