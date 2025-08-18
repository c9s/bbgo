# First stage container
FROM golang:1.23-alpine3.20 AS builder
RUN apk add --no-cache git ca-certificates gcc musl-dev libc-dev pkgconfig
# gcc is for github.com/mattn/go-sqlite3
# ADD . $GOPATH/src/github.com/c9s/bbgo


WORKDIR $GOPATH/src/github.com/c9s/bbgo
ARG GO_MOD_CACHE
ENV WORKDIR=$GOPATH/src/github.com/c9s/bbgo
ENV GOPATH_ORIG=$GOPATH
ENV GOPATH=${GO_MOD_CACHE:+$WORKDIR/$GO_MOD_CACHE}
ENV GOPATH=${GOPATH:-$GOPATH_ORIG}
ENV CGO_ENABLED=1
RUN cd $WORKDIR
ADD . .
RUN go get github.com/mattn/go-sqlite3
RUN go build -o $GOPATH_ORIG/bin/bbgo ./cmd/bbgo

# Second stage container
FROM alpine:3.20

# Create the default user 'bbgo' and assign to env 'USER'
ENV USER=bbgo
RUN adduser -D -G wheel "$USER"
USER ${USER}

COPY --from=builder /go/bin/bbgo /usr/local/bin

WORKDIR /home/${USER}
ENTRYPOINT ["/usr/local/bin/bbgo"]
CMD ["run", "--config", "/config/bbgo.yaml", "--no-compile"]
# vim:filetype=dockerfile:
