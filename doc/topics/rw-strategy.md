## Print/Modify Strategy Fields

The following utility functions are commonly used in the `elliottwave` and `drift` strategies. You can refer to their implementations in `pkg/strategy` for further details.

To handle output or modification of strategy fields, BBGO offers a utility that simplifies parameter serialization, similar to how JSON fields are marshaled. This utility adopts JSON-like tag syntax with additional tags for specific behaviors.

For example:

```go
type Strategy struct {
    // The Debug field will be serialized to JSON with "debug" as the key.
    Debug bool `json:"debug"`
}
```

This utility is located in `github.com/c9s/bbgo/pkg/dynamic`, and the style configuration can be found in `github.com/c9s/bbgo/pkg/style`.

To output the configuration, use `dynamic.PrintConfig`, which only serializes fields that can be marshaled into JSON:

```go
import (
    "io"
    "github.com/c9s/bbgo/pkg/dynamic"
    "github.com/c9s/bbgo/pkg/interact"
    "github.com/jedib0t/go-pretty/v6/table"
)

func (s *Strategy) Print(f io.Writer, pretty bool, withColor ...bool) {
    var tableStyle *table.Style
    if pretty {
        tableStyle = style.NewDefaultTableStyle()
    }
    dynamic.PrintConfig(s, f, tableStyle, len(withColor) > 0 && withColor[0], dynamic.DefaultWhiteList()...)
}
```

We can now register a command to allow users to interact with the strategy:

```go
import (
    "bytes"
    "github.com/c9s/bbgo/pkg/bbgo"
    "github.com/c9s/bbgo/pkg/interact"
)

...
bbgo.RegisterCommand("/config", "Show latest config", func(reply interact.Reply) {
    var buffer bytes.Buffer
    s.Print(&buffer, false)
    reply.Message(buffer.String())
})
```

To dump all strategy fields, you can use `dynamic.ParamDump`. If certain fields should be excluded, simply add the `ignore: "true"` tag to the field definition.

To make fields modifiable, use the `modifiable: "true"` tag and call `bbgo.RegisterModifier(s)` to enable editing. This will automatically add `/modify` commands to the strategy.
