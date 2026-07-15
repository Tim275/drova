FROM golang:1.26-alpine@sha256:111d79159b2326f7e80c4a4706e1ba166acb0e2611df853955f3621828cd49e8 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/search-indexer ./services/search-indexer/cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/search-indexer /app/search-indexer
USER nonroot:nonroot
EXPOSE 8086
ENTRYPOINT ["/app/search-indexer"]
