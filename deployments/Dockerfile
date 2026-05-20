FROM golang:1.24
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/server ./cmd/server
COPY configs/config.example.yaml /app/configs/config.example.yaml
EXPOSE 8080
CMD ["/app/server"]
