#!/bin/bash
#<UDF name="max_api_key" label="MAX API Key"/>
# MAX_API_KEY=
#
#<UDF name="max_api_secret" label="MAX Secret Key"/>
# MAX_API_SECRET=
#
#<UDF name="lower_price" label="Lower price of the grid band" default="28.0" example="28.0" />
# LOWER_PRICE=
#
#<UDF name="upper_price" label="Upper price of the grid band" default="29.0" example="29.0" />
# UPPER_PRICE=
#
#<UDF name="grid_number" label="Number of Grids" default="10" example="10"/>
# GRID_NUMBER=
#
#<UDF name="quantity" label="Quantity" default="10.0" example="10.0"/>
# QUANTITY=
#
#<UDF name="profit_spread" label="Profit Spread" default="0.1" example="0.1"/>
# PROFIT_SPREAD=
#
#<UDF name="side" label="Initial grid side" default="both" oneof="buy,sell,both"/>
# SIDE=
#
#<UDF name="catch_up" label="Catch up price if price raises or drops" default="false" oneof="true,false"/>
# CATCH_UP=
#
#<UDF name="long" label="Keep profit in the base asset" default="true" oneof="true,false"/>
# LONG=
set -e
osf=$(uname | tr '[:upper:]' '[:lower:]')
version=v1.13.0
dist_file=bbgo-$version-$osf-amd64.tar.gz

apt-get install -y redis-server

curl -O -L https://github.com/c9s/bbgo/releases/download/$version/$dist_file
tar xzf $dist_file
mv bbgo-$osf bbgo
chmod +x bbgo
mv bbgo /usr/local/bin/bbgo

useradd --create-home -g users -s /usr/bin/bash bbgo
cd /home/bbgo

cat <<END > .env.local
export MAX_API_KEY=$MAX_API_KEY
export MAX_API_SECRET=$MAX_API_SECRET
END

cat <<END > /etc/systemd/system/bbgo.service
[Unit]
Description=bbgo trading bot
After=network.target

[Install]
WantedBy=multi-user.target

[Service]
WorkingDirectory=/home/bbgo
# EnvironmentFile=/home/bbgo/envvars
ExecStart=/usr/local/bin/bbgo run --enable-web-server
KillMode=process
User=bbgo
Restart=always
RestartSec=10
END

cat <<END > bbgo.yaml
---
persistence:
  json:
    directory: var/data
  redis:
    host: 127.0.0.1
    port: 6379
    db: 0

exchangeStrategies:
- on: max
  grid:
    symbol: USDTTWD
    quantity: $QUANTITY
    gridNumber: $GRID_NUMBER
    profitSpread: $PROFIT_SPREAD
    upperPrice: $UPPER_PRICE
    lowerPrice: $LOWER_PRICE
    side: $SIDE
    long: $LONG
    catchUp: $CATCH_UP
    persistence:
      type: redis
      store: main
END

systemctl enable bbgo.service
systemctl daemon-reload
systemctl start bbgo
