# Build From Source

## Install Go SDK

Go to the Go official website to download the Go SDK <https://go.dev/dl/>.

An example installation looks like this:

```shell
wget https://go.dev/dl/go1.17.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.17.4.linux-amd64.tar.gz
```

Then edit your ~/.profile or ~/.bashrc to have this line at the end:

```shell
export PATH=$PATH:/usr/local/go/bin
```

For the changes to be taken into action, you need to log in again, or run:

```shell
source $HOME/.profile
```

Make sure your `go` is successfully installed:

```shell
go version
```

## Install go-sqlite

If you need to use go-sqlite, you will need to enable CGO first:

```
CGO_ENABLED=1 go get github.com/mattn/go-sqlite3
```

## Install

### Install bbgo via go install

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

### Install via git clone

Since the default GOPATH is located at `~/go`, you can clone the bbgo repo into the folder `~/go/src/github.com/c9s/bbgo`:

```shell
mkdir -p ~/go/src/github.com/c9s
git clone git@github.com:c9s/bbgo.git ~/go/src/github.com/c9s/bbgo
cd ~/go/src/github.com/c9s/bbgo
```

Download the go modules:

```shell
go mod download
```

And then you should be able to run bbgo with `go run`

```shell
go run ./cmd/bbgo run
```

You can also use the makefile to build bbgo:

```shell
cd apps/frontend && yarn install
make bbgo
```

If you don't need the web interface, you can build the slim version of bbgo:

```shell
make bbgo-slim
```

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
