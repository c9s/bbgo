#!/bin/bash
set -e
source $(dirname $(readlink -f $0))/tagname.sh
osf=$(uname | tr '[:upper:]' '[:lower:]')
dist_file=bbgo-dnum-$version-$osf-amd64.tar.gz

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

info "downloading..."
curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-dnum-$osf-amd64 bbgo
chmod +x bbgo
info "downloaded successfully"
