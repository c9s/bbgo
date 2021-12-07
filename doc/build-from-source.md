# Build From Source

Go to the Go official website to download the Go SDK <https://go.dev/dl/>.


Make sure your `go` is successfully installed:

```shell
go version
```

If you need to use go-sqlite, you will need to enable CGO first:

```
CGO_ENABLED=1 go get github.com/mattn/go-sqlite3
```

Install bbgo:

```sh
go install -x github.com/c9s/bbgo/cmd/bbgo@main
```

Your binary will be installed into the default GOPATH `~/go/bin`.
You can add the bin path to your PATH env var by adding the following code to your `~/.zshrc` or `~/.bashrc`:

```shell
export PATH=~/go/bin:$PATH
```

And then, check the version, it should be `1.x-dev`:

```shell
bbgo version
```

If not, try running `ls -lh ~/go/bin/bbgo` to see if the binary is installed.
If it's already there, it means your PATH is misconfigured.

If you prefer other place for installing the go related binaries, you can set GOPATH to somewhere else, e.g.

```shell
export GOPATH=~/mygo
```

Then your bbgo will be installed at `~/mygo/bin/bbgo`.

## Build inside a Alpine container

Starts a docker container with the alpine image:

```shell
docker run -it --rm alpine
```

Run the following command to install the dependencies:

```shell
apk add git go gcc libc-dev sqlite
export CGO_ENABLED=1
go get github.com/mattn/go-sqlite3
go install github.com/c9s/bbgo/cmd/bbgo@latest
```

Your installed bbgo binary will be located in:

```
/root/go/bin/bbgo version
```

You can use the above instruction to write your own Dockerfile.
