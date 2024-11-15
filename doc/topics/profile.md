Profiling
===================

```shell
dotenv -f .env.local -- go run -tags pprof ./cmd/bbgo run \
    --config ./makemoney-btcusdt.yaml \
    --cpu-profile makemoney.pprof \
    --enable-profile-server
```

```shell
go tool pprof http://localhost:6060/debug/pprof/heap
```
