### Setting up Telegram Bot Notification

Open your Telegram app, and chat with @botFather

Enter `/newbot` to create a new bot

Enter the bot display name. ex. `your_bbgo_bot`

Enter the bot username. This should be global unique. e.g., `bbgo_bot_711222333`

Botfather will response your a bot token. *Keep bot token safe*

Set `TELEGRAM_BOT_TOKEN` in the `.env.local` file, e.g.,

```sh
TELEGRAM_BOT_TOKEN=347374838:ABFTjfiweajfiawoejfiaojfeijoaef
```

For the telegram chat authentication (your bot needs to verify it's you), if you only need a fixed authentication token,
you can set `TELEGRAM_AUTH_TOKEN` in the `.env.local` file, e.g.,

```sh
TELEGRAM_BOT_AUTH_TOKEN=itsme55667788
```

Run your bbgo,

Open your Telegram app, search your bot `bbgo_bot_711222333`

Enter `/start` and `/auth {code}`

Done! your notifications will be routed to the telegram chat.