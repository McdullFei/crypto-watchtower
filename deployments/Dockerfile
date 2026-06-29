ARG GO_BUILD_IMAGE=golang:1.24
FROM ${GO_BUILD_IMAGE} AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/server /app/server
COPY configs/config.example.yaml /app/configs/config.example.yaml
COPY migrations /app/migrations
EXPOSE 8080
CMD ["/app/server"]
