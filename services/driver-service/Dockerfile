FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/driver-service ./services/driver-service

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/driver-service /app/driver-service
COPY --from=builder --chown=nonroot:nonroot /app/services/driver-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 9092
ENTRYPOINT ["/app/driver-service"]
