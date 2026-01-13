# name=Dockerfile
FROM golang:1.25-bullseye AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/yourtaskplanner ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /usr/local/bin/yourtaskplanner /usr/local/bin/yourtaskplanner
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/usr/local/bin/yourtaskplanner"]