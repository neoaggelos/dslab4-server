FROM golang:1.14 AS builder
ENV CGO_ENABLED=0
COPY server.go go.mod go.sum /app
WORKDIR /app
RUN go build .

FROM alpine:latest
COPY --from=builder /app/dslab4-server /dslab4-server
ENTRYPOINT ["/dslab4-server"]
