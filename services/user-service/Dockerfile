FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o user-service ./services/user-service/cmd

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/user-service .
COPY --from=builder /app/services/user-service/migrations ./migrations
EXPOSE 8082
CMD ["./user-service"]
