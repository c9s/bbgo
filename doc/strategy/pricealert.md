### Price Alert Strategy

This strategy will send notifications to specified channels when the price change of the specified trading pairs is 
larger than the threshold.


### Prerequisite 
Setup Telegram/Slack notification before using Price Alert Strategy. See [Setting up Telegram Bot Notification
](../configuration/telegram.md) and [Setting up Slack Notification](../configuration/slack.md).


#### Parameters

- `symbol`
    - The trading pair symbol, e.g., `BTCUSDT`, `ETHUSDT`
- `interval`
    - The K-line interval, e.g., `5m`, `1h`
- `minChange`
    - Alert threshold, e.g., `100`, `500`. This is a fixed value of price change. Any price change in a single K-line 
      larger than this value will trigger the alert.  


#### Examples

See [pricealert.yaml](../../config/pricealert.yaml) and [pricealert-tg.yaml](../../config/pricealert-tg.yaml)