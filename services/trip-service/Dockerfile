FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o trip-service ./services/trip-service/cmd

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/trip-service .
EXPOSE 9093
CMD ["./trip-service"]
