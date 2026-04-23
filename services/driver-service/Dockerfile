FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o driver-service ./services/driver-service

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/driver-service .
EXPOSE 9092
CMD ["./driver-service"]
