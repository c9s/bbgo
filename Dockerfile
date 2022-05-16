# First stage container
FROM golang:1.18.2-alpine3.15 AS builder
RUN apk add --no-cache git ca-certificates gcc libc-dev pkgconfig
# gcc is for github.com/mattn/go-sqlite3
# ADD . $GOPATH/src/github.com/c9s/bbgo

WORKDIR $GOPATH/src/github.com/c9s/bbgo
ARG GO_MOD_CACHE
ENV WORKDIR=$GOPATH/src/github.com/c9s/bbgo
ENV GOPATH_ORIG=$GOPATH
ENV GOPATH=${GO_MOD_CACHE:+$WORKDIR/$GO_MOD_CACHE}
ENV GOPATH=${GOPATH:-$GOPATH_ORIG}
ENV CGO_ENABLED=1
RUN go install github.com/mattn/go-sqlite3
ADD . .
RUN go build -o $GOPATH_ORIG/bin/bbgo ./cmd/bbgo

# Second stage container
FROM alpine:3.15

# RUN apk add --no-cache ca-certificates
RUN mkdir /app

WORKDIR /app
COPY --from=builder /go/bin/bbgo /usr/local/bin

ENTRYPOINT ["/usr/local/bin/bbgo"]
CMD ["run", "--config", "/config/bbgo.yaml", "--no-compile"]
# vim:filetype=dockerfile:
