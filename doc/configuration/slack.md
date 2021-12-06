### Setting up Slack Notification

Put your slack bot token in the `.env.local` file:

```sh
SLACK_TOKEN=xxoox
```

And add the following notification config in your `bbgo.yml`:

```yaml
---
notifications:
  slack:
    defaultChannel: "bbgo-xarb"
    errorChannel: "bbgo-error"

  # routing rules
  routing:
    trade: "$silent"
    order: "$slient"
    submitOrder: "$slient"
```
