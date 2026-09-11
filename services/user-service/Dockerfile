FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/user-service ./services/user-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/user-service /app/user-service
COPY --from=builder --chown=nonroot:nonroot /app/services/user-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app/user-service"]
