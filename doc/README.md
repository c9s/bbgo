# BBGO Documentation Index
--------------------------

### General Topics
* [Commands](commands/bbgo.md) - BBGO command line usage
* [Build From Source](build-from-source.md) - How to build bbgo
* [Back-testing](topics/back-testing.md) - How to back-test strategies
* [TWAP](topics/twap.md) - TWAP order execution to buy/sell large quantity of order
* [Dnum Installation](topics/dnum-binary.md) - installation of high-precision version of bbgo
* [bbgo completion](topics/bbgo-completion.md) - Convenient use of the command line

### Configuration
* [Setting up Slack Notification](configuration/slack.md)
* [Setting up Telegram Notification](configuration/telegram.md) - Setting up Telegram Bot Notification
* [Environment Variables](configuration/envvars.md)
* [Syncing Trading Data](configuration/sync.md) - Synchronize private trading data

### Deployment
* [Helm Chart Deployment](deployment/helm-chart.md)
* [Setting up Systemd](deployment/systemd.md)

### Strategies
* [Grid](strategy/grid.md) - Grid Strategy Explanation
* [Interaction](strategy/interaction.md) - Interaction registration for strategies
* [Price Alert](strategy/pricealert.md) - Send price alert notification on price changes
* [Supertrend](strategy/supertrend.md) - Supertrend strategy uses Supertrend indicator as trend, and DEMA indicator as noise filter
* [Support](strategy/support.md) - Support strategy that buys on high volume support

### Development
* [Developing Strategy](topics/developing-strategy.md) - developing strategy
* [Adding New Exchange](development/adding-new-exchange.md) - Check lists for adding new exchanges
* [KuCoin Command-line Test Tool](development/kucoin-cli.md) - Kucoin command-line tools
* [SQL Migration](development/migration.md) - Adding new SQL migration scripts
* [Release Process](development/release-process.md) - How to make a new release
