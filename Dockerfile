# First stage container
FROM golang:1.15-alpine3.12 AS builder
RUN apk add --no-cache git ca-certificates gcc libc-dev pkgconfig
# gcc is for github.com/mattn/go-sqlite3
RUN go get -u github.com/c9s/goose/cmd/goose
ADD . $GOPATH/src/github.com/c9s/bbgo
WORKDIR $GOPATH/src/github.com/c9s/bbgo
# RUN GOPATH=$PWD/.mod go install ./cmd/bbgo
RUN go install ./cmd/bbgo

# Second stage container
FROM alpine:3.12

# RUN apk add --no-cache ca-certificates
RUN mkdir /app

WORKDIR /app
COPY --from=builder /go/bin/goose /usr/local/bin
COPY --from=builder /go/bin/bbgo /usr/local/bin

ENTRYPOINT ["/usr/local/bin/bbgo"]
CMD ["run", "--config", "/config/bbgo.yaml"]
# vim:filetype=dockerfile:
