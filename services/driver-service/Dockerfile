FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/driver-service ./services/driver-service/cmd

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/driver-service /app/driver-service
COPY --from=builder --chown=nonroot:nonroot /app/services/driver-service/migrations /app/migrations
USER nonroot:nonroot
EXPOSE 9092
ENTRYPOINT ["/app/driver-service"]
