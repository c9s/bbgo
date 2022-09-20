### Setting up Slack Notification

Go to the Slack apps page to create your own slack app:

<https://api.slack.com/apps>

Click "Install your app" -> "Install to Workspace" in *Settings/Basic Information*.

Copy the *Bot User OAuth Token* in *Features/OAuth & Permissions*.

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

  switches:
    trade: true
    orderUpdate: true
    submitOrder: true
```

Besure to add your bot to the public channel by clicking "Add slack app to channel".

## See Also

- <https://www.ibm.com/docs/en/z-chatops/1.1.0?topic=slack-adding-your-bot-user-your-channel>
