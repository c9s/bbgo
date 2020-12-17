# First stage container
FROM golang:1.15-alpine3.12 AS builder
RUN apk add --no-cache git ca-certificates gcc libc-dev pkgconfig
# gcc is for github.com/mattn/go-sqlite3
RUN go get -u github.com/c9s/goose/cmd/goose
ADD . $GOPATH/src/github.com/c9s/bbgo
WORKDIR $GOPATH/src/github.com/c9s/bbgo
ARG GO_MOD_CACHE
ENV GOPATH_ORIG=$GOPATH
ENV GOPATH=${GO_MOD_CACHE:+$PWD/$GO_MOD_CACHE}
ENV GOPATH=${GOPATH:-$GOPATH_ORIG}
RUN go build -o $GOPATH_ORIG/bin/bbgo ./cmd/bbgo

# Second stage container
FROM alpine:3.12

# RUN apk add --no-cache ca-certificates
RUN mkdir /app

WORKDIR /app
COPY --from=builder /go/bin/goose /usr/local/bin
COPY --from=builder /go/bin/bbgo /usr/local/bin

ENTRYPOINT ["/usr/local/bin/bbgo"]
CMD ["run", "--config", "/config/bbgo.yaml", "--no-compile"]
# vim:filetype=dockerfile:
