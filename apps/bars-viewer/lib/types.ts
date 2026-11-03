// KLine mirrors the JSON shape of pkg/types.KLine on the bars-backend, used
// for both regular candles and indicator/v2 tick/volume bars (which are also
// emitted as KLine).
export interface KLine {
  exchange: string
  symbol: string
  startTime: string
  endTime: string
  interval: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  quoteVolume: number
  closed: boolean
}

// WSMessage is the envelope bars-backend wraps every /ws/klines and /ws/bars
// push in, so the client can tell a historical snapshot entry, the end of the
// snapshot, and a live update apart.
export interface WSMessage {
  type: 'snapshot' | 'snapshotEnd' | 'update'
  data?: KLine
}

export type BarType = 'tick' | 'volume'
