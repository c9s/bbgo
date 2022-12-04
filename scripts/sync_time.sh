#!/usr/bin/sh

date -s @`echo "$(curl https://api.binance.com/api/v3/time 2>/dev/null | jq .serverTime)/1000"|bc -l`
