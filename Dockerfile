# 构建阶段
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook .

# 运行阶段
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/webhook .

EXPOSE 8443

CMD ["./webhook"]