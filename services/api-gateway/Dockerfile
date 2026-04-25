FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/api-gateway ./services/api-gateway

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/api-gateway /app/api-gateway
COPY --from=builder --chown=nonroot:nonroot /app/frontend /app/frontend
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/app/api-gateway"]
