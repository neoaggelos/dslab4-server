FROM golang:1.14 AS builder
ENV CGO_ENABLED=0
RUN go get -u github.com/neoaggelos/dslab4-server

FROM alpine:latest
COPY --from=builder /app/dslab4-server /dslab4-server
ENTRYPOINT ["/dslab4-server"]
