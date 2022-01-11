# Interaction

In your strategy, you can register your messenger interaction by commands.


```
package mymaker

import (
    "github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
    bbgo.RegisterInteraction(&MyInteraction{})
}

type MyInteraction struct {}

func (m *MyInteraction) Commands(interact bbgo.Interact) {
    interact.Command("closePosition", func(w bbgo.InteractWriter, symbol string, percentage float64) {
    
    })
}


type Strategy struct {}
```


The interaction engine parses the command from the messenger software programs like Telegram or Slack.
And then pass the arguments to the command handler defined in the strategy.

