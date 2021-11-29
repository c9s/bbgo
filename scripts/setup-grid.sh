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

osf=$(uname | tr '[:upper:]' '[:lower:]')
arch=amd64
version=v1.18.0
dist_file=bbgo-$version-$osf-$arch.tar.gz

info "downloading..."
curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-$osf-$arch bbgo
chmod +x bbgo
info "downloaded successfully"

function gen_dotenv()
{
    read -p "Enter your MAX API key: " api_key
    read -p "Enter your MAX API secret: " api_secret
    info "generating your .env.local file..."
cat <<END > .env.local
MAX_API_KEY=$api_key
MAX_API_SECRET=$api_secret
END

    info "dotenv is configured successfully"
}

if [[ -e ".env.local" ]] ; then
    warn "found an existing .env.local, you will overwrite the existing .env.local file!"
    read -p "are you sure? (Y/n) " a
    if [[ $a != "n" ]] ; then
        gen_dotenv
    fi
else
    gen_dotenv
fi


if [[ -e "bbgo.yaml" ]] ; then
  warn "found existing bbgo.yaml, you will overwrite the existing bbgo.yaml file!"
  read -p "are you sure? (Y/n) " a
  if [[ $a == "n" ]] ; then
    exit
  fi
fi

cat <<END > bbgo.yaml
---
riskControls:
  sessionBased:
    max:
      orderExecutor:
        bySymbol:
          BTCUSDT:
            # basic risk control order executor
            basic:
              minQuoteBalance: 100.0
              maxBaseAssetBalance: 3.0
              minBaseAssetBalance: 0.0
              maxOrderAmount: 1000.0

exchangeStrategies:
- on: max
  grid:
    symbol: BTCUSDT
    quantity: 0.001
    gridNumber: 100
    profitSpread: 100.0
    upperPrice: 50_000.0
    lowerPrice: 10_000.0
    long: true

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
