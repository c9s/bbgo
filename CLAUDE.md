# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BBGO is a modern cryptocurrency trading bot framework written in Go. It provides exchange abstraction, real-time market data streaming, strategy development tools, back-testing engine, and supports multiple exchanges including Binance, OKX, Kucoin, MAX, Bitget, Bybit, and Coinbase.

## Development Commands

### Building
- `make bbgo` - Build native bbgo with web frontend
- `make bbgo-slim` - Build slim version without web frontend
- `make bbgo-linux` - Cross-compile for Linux (amd64 and arm64)
- `make bbgo-darwin` - Cross-compile for macOS (amd64 and arm64)
- `make bbgo-dnum` - Build with high precision decimal support (16-digit precision)

### Frontend Development
- `make static` - Build frontend assets and embed them
- Frontend is located in `apps/frontend/` using Next.js
- Backtest reports in `apps/backtest-report/` also using Next.js

### Testing
- `go test ./...` - Run all tests
- Test files follow Go conventions (`*_test.go`)
- Test helpers available in `pkg/testing/testhelper/`

### Database
- `make migrations` - Compile database migrations
- Supports MySQL and SQLite3
- Migration files in `migrations/mysql/` and `migrations/sqlite3/`

### Running
- `bbgo run --config config.yaml` - Run with configuration
- `bbgo run --enable-webserver` - Run with web dashboard
- `bbgo backtest --config config.yaml` - Run backtesting

## Architecture

### Core Components

**Environment** (`pkg/bbgo/environment.go`)
- Central orchestrator managing all services and sessions
- Handles database connections, sync services, and exchange sessions
- Manages profiling, notifications, and user data streaming

**Exchange Sessions** (`pkg/bbgo/session.go`)
- Wrapper around exchange APIs providing unified interface
- Handles market data streams, user data streams, and account management
- Each session represents a connection to one exchange with specific configuration

**Strategies** (`pkg/strategy/`)
- Organized by strategy name in subdirectories
- Implement `SingleExchangeStrategy` or `CrossExchangeStrategy` interfaces
- Auto-registered via `RegisterStrategy()` function

**Services** (`pkg/service/`)
- Database services for orders, trades, profits, positions
- Sync services for historical data
- Persistence services for strategy state

### Key Packages

- `pkg/types/` - Core data types and interfaces
- `pkg/exchange/` - Exchange-specific implementations
- `pkg/indicator/` - Technical analysis indicators
- `pkg/bbgo/` - Core framework logic
- `pkg/backtest/` - Backtesting engine
- `pkg/cmd/` - CLI command implementations

### Exchange Support
Currently supports: Binance, OKX, Kucoin, MAX, Bitget, Bybit, Coinbase. Each exchange implementation in `pkg/exchange/{name}/`.

### Strategy Development
1. Create strategy in `pkg/strategy/{name}/strategy.go`
2. Implement required interfaces (`SingleExchangeStrategy` or `CrossExchangeStrategy`)
3. Register strategy using `RegisterStrategy()`
4. Strategies can use dependency injection for sessions, markets, and services

### Configuration
- Main config in YAML format
- Environment variables for API keys and database settings
- Strategy-specific configuration sections
- Example configs in `config/` directory

### Data Persistence
- Uses database for historical data (trades, orders, profits)
- Redis for session persistence
- JSON/Memory for strategy state

### Notifications
- Slack integration with channel configuration
- Telegram bot support with authentication
- Configurable notification switches for different events

This framework emphasizes modularity, allowing developers to focus on strategy logic while handling exchange connectivity, data management, and execution infrastructure automatically.