# MTF TTM Squeeze Momentum Strategy

This strategy combines **Volatility Compression (Squeeze)** with **Multi-Timeframe (MTF) Trend Filtering** to capture explosive moves when the market shifts from a period of consolidation to a trending phase. By using the **21 EMA** as a dynamic baseline, we ensure we only trade in the direction of the institutional trend.

---

## Overview
The core philosophy of this strategy is **"Big Wave, Small Wave."** We use a higher timeframe (HTF) to determine the "tide" (trend) and a lower timeframe (LTF) to catch the "wave" (entry). The 21 EMA serves as the "Line in the Sand"—we only look for long opportunities when the price is trading above this level on both timeframes.

## Technical Components
* **TTM Squeeze:** Identifies periods of low volatility (Black/Red/Orange dots) and the subsequent breakout (Green dots).
* **Momentum Histogram:** A linear regression-based oscillator that shows the strength and direction of the move.
* **21 EMA (Exponential Moving Average):** Acts as the primary trend filter and dynamic support level.

---

## Strategy Rules

### Trend Filter (High Timeframe - e.g., Daily)
Before looking for an entry on the LTF, the following conditions must be met on the HTF:
* **Price Action:** Current Price > 21 EMA.
* **Momentum:** The HTF Momentum Histogram should ideally be positive or at least bottoming out (Yellow or Cyan).

### Entry Conditions (Low Timeframe - e.g., 1H or 4H)
Enter a **Long** position when all the following align:
1.  **Squeeze Fired:** The Squeeze dots turn from **Red/Black/Orange to Green** (The "Fire" signal).
2.  **Momentum Confirmation:** The Histogram is **Cyan** (Above zero and increasing).
3.  **Trend Alignment:** Price is trading above the **LTF 21 EMA**.
4.  **HTF Confirmation:** The HTF trend filter (Condition A) remains bullish.

### Exit Strategy
To maximize gains while protecting capital, we use multiple exit triggers:

**Exit Conditions (Close Entire Position):**
1.  **Max Loss Exceeded (Hard Exit):**
    * **Trigger:** When ROI drops below the negative `exit.maxLossRatio` (e.g., ROI < -10%).
    * **Logic:** Risk management to prevent excessive drawdown. Uses **immediate market orders** (not TWAP) for fastest execution.
    * **Cooldown:** After a hard exit, no new entries are allowed for `exit.hardExitCoolDown` duration (default: 1 hour).
2.  **Price Below EMA:**
    * **Trigger:** When the Price **closes below the LTF 21 EMA**.
    * **Logic:** This signals that the short-term trend has officially broken.
3.  **Momentum Weakening:**
    * **Trigger:** When the LTF Momentum Histogram shows **consecutive BullishSlowing signals** (configurable via `exit.consecutiveExit`).
    * **Logic:** Exit when momentum fades consistently, avoiding whipsaws from single weak signals.

### Stop Loss (Risk Management)
* **Initial Stop:** Place the stop loss below the **lowest point of the Squeeze consolidation period** (the "Red dot" zone).
* **Max Loss Exit:** The `exit.maxLossRatio` configuration provides automatic stop-loss protection based on position ROI.

---

## Lifecycle Behavior

### Startup
* **Open Order Cleanup:** On startup, the strategy cancels all open orders for the symbol from previous runs to ensure a clean state.
* **State Recovery:** The state machine checks current position and recovers to the appropriate state (Idle/Long).

### Shutdown
* **Graceful Cancellation:** All pending orders are gracefully cancelled.
* **Position Clearing:** If `exit.clearPositionOnShutdown` is enabled, the strategy closes any open position using multiple market orders.
* **Final Cleanup:** Any remaining open orders are cancelled via REST API as a safety check.

---

## Strategy Summary Table

| Step | Action | Condition |
| :--- | :--- | :--- |
| **1. Filter** | Define Trend | HTF Price > 21 EMA |
| **2. Setup** | Wait for Energy | LTF shows Red/Black/Orange dots (Squeeze On) |
| **3. Entry** | Execute Buy | LTF First Green Dot + Cyan Histogram + Price > 21 EMA (requires `entry.consecutiveEntries` signals) |
| **4. Exit** | **Full Exit** | Price closes below **LTF 21 EMA** OR ROI exceeds `exit.maxLossRatio` OR consecutive momentum weakening |

---

## Key Advantages
* **High Probability:** By requiring HTF alignment, you avoid "trading into a wall."
* **Volatility Protected:** Entering on the "Green Dot" ensures you aren't stuck in a sideways market for weeks.
* **Consecutive Signal Filtering:** Requiring multiple consecutive signals (configurable) reduces false entries and exits from noise.
* **TWAP Execution:** All entries and exits use TWAP to minimize market impact and achieve better average prices.
* **Clean State Management:** Automatic cleanup of open orders on startup and shutdown prevents orphaned orders.

## Configuration

```yaml
exchangeStrategies:
- on: binance
  mtf-ttmsqueeze:
    symbol: BTCUSDT
    ltfInterval: 15m           # Lower timeframe for signals
    htfInterval: 1d            # Higher timeframe for trend
    window: 20                 # Squeeze length (default: 20)
    emaWindow: 21              # EMA trend length (default: 21)

    # Entry configuration (required)
    entry:
      maxPosition: 1.0         # Maximum total position size (required)
      quantity: 0.1            # Max quantity per entry (required)
      consecutiveEntries: 3    # Consecutive entry signals required (default: 3)
      numOrders: 10            # Number of TWAP orders for entry (default: 10)

    # Exit configuration
    exit:
      maxLossRatio: 0.10       # Hard exit when ROI < -10% (default: 10%)
      hardExitCoolDown: 1h     # Cooldown after hard exit (default: 1h)
      hardExitCheckInterval: 5m # Interval for checking max loss (default: 5m)
      consecutiveExit: 3       # Consecutive momentum weakening signals for exit (default: 3)
      numOrders: 10            # Number of TWAP orders for exit (default: 10)
      clearPositionOnShutdown: false # Close position on shutdown (default: false)

    # TWAP execution configuration
    twap:
      updateInterval: 10s      # TWAP order update interval (default: 10s)
      numOfTicks: 2            # Price ticks behind best bid/ask (default: 2)
      deadline: 5m             # Maximum TWAP execution time (default: 5m)
```

### Configuration Details

#### Entry Configuration
| Field | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `maxPosition` | Yes | - | Maximum total position size allowed |
| `quantity` | Yes | - | Maximum quantity per entry order |
| `consecutiveEntries` | No | 3 | Number of consecutive entry signals required before entering |
| `numOrders` | No | 10 | Number of TWAP orders to execute when entering |

#### Exit Configuration
| Field | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `maxLossRatio` | No | 0.10 | Maximum loss ratio before hard exit (10% = 0.10), based on position ROI |
| `hardExitCoolDown` | No | 1h | Cooldown duration after a hard exit before trading resumes |
| `hardExitCheckInterval` | No | 5m | Interval for checking if position ROI exceeds max loss |
| `consecutiveExit` | No | 3 | Consecutive momentum weakening signals required for exit |
| `numOrders` | No | 10 | Number of TWAP orders to execute when exiting |
| `clearPositionOnShutdown` | No | false | If true, close any open position with market orders on shutdown |

#### TWAP Configuration
| Field | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `updateInterval` | No | 10s | Interval between TWAP order updates |
| `numOfTicks` | No | 2 | Price ticks behind best bid/ask |
| `deadline` | No | 5m | Maximum duration for TWAP execution |
