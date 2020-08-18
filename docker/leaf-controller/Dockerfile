FROM golang:1.15-alpine3.12 AS builder
WORKDIR $GOPATH/src/github.com/wallyqs/satellite/
RUN apk add --update git
COPY . .
RUN go get ./...
RUN CGO_ENABLED=0 go build -installsuffix cgo -o /nats-leaf-controller ./cmd/leaf-controller/main.go

FROM alpine:3.12
COPY --from=builder /nats-leaf-controller /usr/local/bin/nats-leaf-controller
CMD ["nats-leaf-controller"]
