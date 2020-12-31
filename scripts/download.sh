#!/bin/bash
osf=$(uname | tr '[:upper:]' '[:lower:]')
version=v1.6.0

if [[ -n $1 ]] ; then
    version=$1
fi

echo "downloading bbgo $version"
curl -L -o bbgo https://github.com/c9s/bbgo/releases/download/$version/bbgo-$osf
chmod +x bbgo

echo "bbgo is downloaded at ./bbgo"
