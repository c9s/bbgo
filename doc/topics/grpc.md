# GRPC server

## Integrating GRPC services

### Install Evans

```shell
brew install evans 
```

Start your bbgo with gRPC server option:

```shell
go run ./cmd/bbgo run --config grid_kucoin.yaml  --debug --enable-grpc
```

The gRPC server port is located at 50051 (default port), you can use evans to connect to the gRPC server:

```shell
evans --host localhost --port 50051 -r repl
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


You can send the request via evans:

```shell
evans -r cli call --file evans/userDataService/subscribe.json  bbgo.UserDataService.Subscribe
evans -r cli call --file evans/marketDataService/subscribe_kline.json  bbgo.MarketDataService.Subscribe
```



