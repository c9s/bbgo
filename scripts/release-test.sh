#!/bin/bash
set -e
dotenv -f .env.local.mysql -- go run ./cmd/bbgo sync --session binance  --config config/sync.yaml
dotenv -f .env.local.sqlite -- go run ./cmd/bbgo sync --session binance  --config config/sync.yaml
