# Developing Strategy

There are two types of strategies in BBGO:

1. built-in strategy: like grid, bollmaker, pricealert strategies, which are included in the pre-compiled binary.
2. external strategy: custom or private strategies that you don't want to expose to public.

For built-in strategies, they are placed in `pkg/strategy` of the BBGO source repository.

For external strategies, you can create a private repository as an isolated go package and place your strategy inside
it.

In general, strategies are Go struct, placed in Go package.

## The Strategy Struct

BBGO loads the YAML config file and re-unmarshal the settings into your struct as JSON string, so you can define the
json tag to get the settings from the YAML config.

For example, if you're writing a strategy in a package called `short`, to load the following config:

```yaml
externalStrategies:
- on: binance
  short:
    symbol: BTCUSDT
```

You can write the following struct to load the symbol setting:

```
package short

type Strategy struct {
    Symbol string `json:"symbol"`
}

```

To use the Symbol setting, you can get the value from the Run method of the strategy:

```
func (s *Strategy) Run(ctx context.Context, session *bbgo.ExchangeSession) error {
    // you need to import the "log" package
    log.Println("%s", s.Symbol)
    return nil
}
```

Now you have the Go struct and the Go package, but BBGO does not know your strategy,
so you need to register your strategy.

Define an ID const in your package:

```
const ID = "short"
```

Then call bbgo.RegisterStrategy with the ID you just defined and a struct reference:

```
func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}
```

Note that you don't need to fill the fields in the struct, BBGO just need to know the type of struct.

(BBGO use reflect to parse the fields from the given struct and allocate a new struct object from the given struct type internally)


## Built-in Strategy










