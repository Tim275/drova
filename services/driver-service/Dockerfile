FROM golang:1.26-alpine@sha256:bf9573d7c1d2b09992e4f893ea1ef30842854846bdb8ae390468f95ea6b09062 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/driver-service ./services/driver-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/driver-service /app/driver-service
COPY --from=builder --chown=nonroot:nonroot /app/services/driver-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 9092
ENTRYPOINT ["/app/driver-service"]
