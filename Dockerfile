FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

EXPOSE 8080

RUN go build -o main ./main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/. .

EXPOSE 8080

CMD ["./main"]