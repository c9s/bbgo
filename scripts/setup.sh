#!/bin/bash
osf=$(uname | tr '[:upper:]' '[:lower:]')
version=v1.1.0

echo "Downloading bbgo"
curl -L -o bbgo https://github.com/c9s/bbgo/releases/download/$version/bbgo-$osf
chmod +x bbgo
echo "Binary downloaded"

if [[ -e "bbgo.yaml" ]] ; then
  echo "Found existing bbgo.yaml, you will overwrite the existing bbgo.yaml file!"
  read -p "Are you sure? (Y/n) " a
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
    quantity: 0.002
    gridNumber: 100
    profitSpread: 50.0
    upperPrice: 14000.0
    lowerPrice: 11000.0
END

echo "Config file is generated"

if [[ -e ".env.local" ]] ; then
  echo "Found existing .env.local, you will overwrite the existing .env.local file!"
  read -p "Are you sure? (Y/n) " a
  if [[ $a == "n" ]] ; then
    exit
  fi
fi

read -p "Enter your MAX API key: " api_key

read -p "Enter your MAX API secret: " api_secret

echo "Generating your .env.local file..."
cat <<END > .env.local
export MAX_API_KEY=$api_key
export MAX_API_SECRET=$api_secret
END

echo "Now you can edit your strategy config file bbgo.yaml to run bbgo"

if [[ $osf == "darwin" ]] ; then
    echo "We found you're using MacOS, you can type:"
    echo ""
    echo "  open -a TextEdit bbgo.yaml"
    echo ""
fi

echo "To run bbgo just type: "
echo ""
echo "   source .env.local && ./bbgo run --config bbgo.yaml"
echo ""
echo "To stop bbgo, just hit CTRL-C"

if [[ $osf == "darwin" ]] ; then
    open -a TextEdit bbgo.yaml
fi

