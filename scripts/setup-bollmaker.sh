#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

function warn()
{
    echo -e "${YELLOW}$@${NC}"
}

function error()
{
    echo -e "${RED}$@${NC}"
}

function info()
{
    echo -e "${GREEN}$@${NC}"
}
version=$(curl -fs https://api.github.com/repos/c9s/bbgo/releases/latest | awk -F '"' '/tag_name/{print $4}')
osf=$(uname | tr '[:upper:]' '[:lower:]')
arch=""
case $(uname -m) in
  x86_64 | ia64) arch="amd64";;
  arm64 | aarch64 | arm) arch="arm64";;
  *)
    echo "unsupported architecture: $(uname -m)"
    exit 1;;
esac
dist_file=bbgo-$version-$osf-$arch.tar.gz

info "downloading..."
curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-$osf-$arch bbgo
chmod +x bbgo
info "downloaded successfully"

function gen_dotenv()
{
    read -p "Enter your Binance API key: " api_key
    read -p "Enter your Binance API secret: " api_secret
    echo "Generating your .env.local file..."
cat <<END > .env.local
BINANCE_API_KEY=$api_key
BINANCE_API_SECRET=$api_secret
END

}

if [[ -e ".env.local" ]] ; then
    echo "Found existing .env.local, you will overwrite the existing .env.local file!"
    read -p "Are you sure? (Y/n) " a
    if [[ $a != "n" ]] ; then
        gen_dotenv
    fi
else
    gen_dotenv
fi

if [[ -e "bbgo.yaml" ]] ; then
  echo "Found existing bbgo.yaml, you will overwrite the existing bbgo.yaml file!"
  read -p "Are you sure? (Y/n) " a
  if [[ $a == "n" ]] ; then
    exit
  fi
fi

cat <<END > bbgo.yaml
---
sessions:
  binance:
    exchange: binance
    envVarPrefix: BINANCE

persistence:
  redis:
    host: 127.0.0.1
    port: 6379
    db: 0

exchangeStrategies:
- on: binance
  bollmaker:
    symbol: ETHUSDT

    # interval is how long do you want to update your order price and quantity
    interval: 1m

    # quantity is the base order quantity for your buy/sell order.
    quantity: 0.05

    # useTickerPrice use the ticker api to get the mid price instead of the closed kline price.
    # The back-test engine is kline-based, so the ticker price api is not supported.
    # Turn this on if you want to do real trading.
    useTickerPrice: false

    # spread is the price spread from the middle price.
    # For ask orders, the ask price is ((bestAsk + bestBid) / 2 * (1.0 + spread))
    # For bid orders, the bid price is ((bestAsk + bestBid) / 2 * (1.0 - spread))
    # Spread can be set by percentage or floating number. e.g., 0.1% or 0.001
    spread: 0.09%

    # minProfitSpread is the minimal order price spread from the current average cost.
    # For long position, you will only place sell order above the price (= average cost * (1 + minProfitSpread))
    # For short position, you will only place buy order below the price (= average cost * (1 - minProfitSpread))
    minProfitSpread: 0.5%

    # dynamicExposurePositionScale overrides maxExposurePosition
    # for domain,
    #   -1 means -100%, the price is on the lower band price.
    #      if the price breaks the lower band, a number less than -1 will be given.
    #   1 means 100%, the price is on the upper band price.
    #      if the price breaks the upper band, a number greater than 1 will be given, for example, 1.2 for 120%, and 1.3 for 130%.
    dynamicExposurePositionScale:
      byPercentage:
        # exp means we want to use exponential scale, you can replace "exp" with "linear" for linear scale
        exp:
          # from lower band -100% (-1) to upper band 100% (+1)
          domain: [ -1, 1 ]
          # when in down band, holds 1.0 by maximum
          # when in up band, holds 0.05 by maximum
          range: [ 10.0, 1.0 ]

    # DisableShort means you can don't want short position during the market making
    # THe short here means you might sell some of your existing inventory.
    disableShort: true

    # uptrendSkew, like the strongUptrendSkew, but the price is still in the default band.
    uptrendSkew: 0.8

    # downtrendSkew, like the strongDowntrendSkew, but the price is still in the default band.
    downtrendSkew: 1.2

    defaultBollinger:
      interval: "1h"
      window: 21
      bandWidth: 2.0

    # neutralBollinger is the smaller range of the bollinger band
    # If price is in this band, it usually means the price is oscillating.
    neutralBollinger:
      interval: "5m"
      window: 21
      bandWidth: 2.0

    # tradeInBand: when tradeInBand is set, you will only place orders in the bollinger band.
    tradeInBand: false

    # buyBelowNeutralSMA: when this set, it will only place buy order when the current price is below the SMA line.
    buyBelowNeutralSMA: false

    persistence:
      type: redis

END

info "config file is generated successfully"
echo "================================================================"
echo "now you can edit your strategy config file bbgo.yaml to run bbgo"

if [[ $osf == "darwin" ]] ; then
    echo "we found you're using MacOS, you can type:"
    echo ""
    echo "  open -a TextEdit bbgo.yaml"
    echo ""
else
    echo "you look like a pro user, you can edit the config by:"
    echo ""
    echo "  vim bbgo.yaml"
    echo ""
fi

echo "To run bbgo just type: "
echo ""
echo "   ./bbgo run"
echo ""
echo "To stop bbgo, just hit CTRL-C"

if [[ $osf == "darwin" ]] ; then
    open -a TextEdit bbgo.yaml
fi
