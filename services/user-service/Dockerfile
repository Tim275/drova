FROM golang:1.25-alpine@sha256:1e0126852075c9c60731c8ba49088448b91f63e2aed97ca9d1a9791622a05946 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/user-service ./services/user-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/user-service /app/user-service
COPY --from=builder --chown=nonroot:nonroot /app/services/user-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app/user-service"]
