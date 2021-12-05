# Build From Source

If you need to use go-sqlite, you will need to enable CGO first:

```
CGO_ENABLED=1 go get github.com/mattn/go-sqlite3
```

If you prefer to put go related binary in your HOME directory:

Install the bbgo command:

```sh
go get github.com/c9s/bbgo/cmd/bbgo@latest
```