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
exchange=max

if [[ -n $1 ]] ; then
    exchange=$1
fi

exchange_upper=$(echo -n $exchange | tr 'a-z' 'A-Z')

info "downloading..."
curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-$osf-$arch bbgo
chmod +x bbgo
info "downloaded successfully"

function gen_dotenv()
{
    read -p "Enter your $exchange_upper API key: " api_key
    read -p "Enter your $exchange_upper API secret: " api_secret
    info "Generating your .env.local file..."
cat <<END > .env.local
${exchange_upper}_API_KEY=$api_key
${exchange_upper}_API_SECRET=$api_secret
END

    info "dotenv is configured successfully"
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
exchangeStrategies:
- on: ${exchange}
  bollgrid:
    symbol: BTCUSDT
    interval: 1h
    gridNumber: 20
    quantity: 0.001
    profitSpread: 100.0

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
