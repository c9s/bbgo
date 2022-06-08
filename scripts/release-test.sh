#!/bin/bash
set -e
echo "testing sync..."
dotenv -f .env.local.mysql -- go run ./cmd/bbgo sync --session binance  --config config/sync.yaml
dotenv -f .env.local.sqlite -- go run ./cmd/bbgo sync --session binance  --config config/sync.yaml

echo "backtest sync..."
echo "backtest mysql sync..."
dotenv -f .env.local.mysql -- go run ./cmd/bbgo backtest --config config/dca.yaml  --sync --sync-only --verify

echo "backtest sqlite sync..."
dotenv -f .env.local.sqlite -- go run ./cmd/bbgo backtest --config config/dca.yaml  --sync --sync-only --verify
