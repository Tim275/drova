FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/user-service ./services/user-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/user-service /app/user-service
COPY --from=builder --chown=nonroot:nonroot /app/services/user-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app/user-service"]
