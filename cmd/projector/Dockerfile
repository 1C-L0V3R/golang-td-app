FROM golang:1.26.4-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/cmd/projector/exe /app/cmd/projector

FROM alpine:3.24.1
WORKDIR /app
COPY --from=builder /app/cmd/projector/exe /app
ENV PROJECT_ROOT=/app
CMD ["/app/exe"]
