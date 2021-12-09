### Setting up Telegram Bot Notification

Open your Telegram app, and chat with @botFather

Enter `/newbot` to create a new bot

Enter the bot display name. ex. `your_bbgo_bot`

Enter the bot username. This should be global unique. e.g., `bbgo_bot_711222333`

Botfather will response your a bot token. *Keep bot token safe*

Add `TELEGRAM_BOT_TOKEN` in your `.env.local` file, e.g.,

```shell
TELEGRAM_BOT_TOKEN=347374838:ABFTjfiweajfiawoejfiaojfeijoaef
```

For the telegram chat authentication (your bot needs to verify it's you), if you only need a fixed authentication token,
you can set `TELEGRAM_AUTH_TOKEN` in the `.env.local` file, e.g.,

```sh
TELEGRAM_BOT_AUTH_TOKEN=itsme55667788
```
 
The alerting strategies use Telegram bot notification without further configuration. You can check the [pricealert
yaml file](./config/pricealert-tg.yaml) in the `config/` directory for example.

If you want the order submitting/filling notification, add the following to your `bbgo.yaml`:

```yaml
notifications:
  routing:
    trade: "$symbol"
    order: "$symbol"
    submitOrder: "$session"
    pnL: "bbgo-pnl"
```

Run your bbgo.

Open your Telegram app, search your bot `bbgo_bot_711222333`

Enter `/start` and `/auth {code}`

Done! Your notifications will be routed to the telegram chat.

## Authenticating yourself with OTP

BBGO supports one-time password (OTP) authentication for Telegram, so you can auth yourself by the one-time password.

When you run your bbgo with the telegram token first time, it will generate an otp token in a PNG file (named otp-xxxx.png) and also the console output.

You should store the otp token in a safe place like 1Password.

In order to save the OTP secret persistently, you should configure your BBGO with redis, simply add the following config to your `bbgo.yaml`:

```yaml
persistence:
  json:
    directory: var/data
  redis:
    host: 127.0.0.1
    port: 6379
    db: 0
```
