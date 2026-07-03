FROM golang:1.26.4-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/cmd/todoapp/exe /app/cmd/todoapp

FROM alpine:3.24.1
WORKDIR /app
COPY --from=builder /app/cmd/todoapp/exe /app
CMD ["/app/exe"]