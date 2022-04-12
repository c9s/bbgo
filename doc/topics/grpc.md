# GRPC server

## Integrating GRPC services

### Install Evans

```shell
brew tap ktr0731/evans
brew install evans
```

Start your bbgo with gRPC server option:

```shell
go run ./cmd/bbgo run --config grid_kucoin.yaml  --debug --enable-grpc
```

The gRPC server port is located at 6688, you can use evans to connect to the gRPC server:

```shell
evans --host localhost --port 6688 -r repl
```

```shell
bbgo@localhost:6688> package bbgo
bbgo@localhost:6688> show service
bbgo@localhost:6688> show message
```

You can use evans to get the description of a message:

```shell
bbgo@localhost:6688> desc QueryKLinesRequest
+-----------+-------------+----------+
|   FIELD   |    TYPE     | REPEATED |
+-----------+-------------+----------+
| exchange  | TYPE_STRING | false    |
| interval  | TYPE_STRING | false    |
| limit     | TYPE_INT64  | false    |
| symbol    | TYPE_STRING | false    |
| timestamp | TYPE_INT64  | false    |
+-----------+-------------+----------+
```






