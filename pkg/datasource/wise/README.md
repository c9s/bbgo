# Wise

[Wise API Docs](https://docs.wise.com/api-docs)

```go
c := wise.NewClient()
c.Auth(os.Getenv("WISE_TOKEN"))

ctx := context.Background()
rates, err := c.QueryRate(ctx, "USD", "TWD")
if err != nil {
    panic(err)
}
fmt.Printf("%+v\n", rates)

// or
now := time.Now()
rates, err = c.QueryRateHistory(ctx, "USD", "TWD", now.Add(-time.Hour*24*7), now, types.Interval1h)
if err != nil {
    panic(err)
}
for _, rate := range rates {
    fmt.Printf("%+v\n", rate)
}
```
