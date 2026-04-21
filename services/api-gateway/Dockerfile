FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o api-gateway ./services/api-gateway

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/api-gateway .
EXPOSE 8081
CMD ["./api-gateway"]
