import { useEffect, useRef, useState } from 'react'
import { KLine, WSMessage } from '../lib/types'

export interface KLineSeriesUpdate {
  // history is set once, right after the snapshot is fully received; the
  // chart should call series.setData(history) when it changes.
  history: KLine[]
  // latest is set on every live update after the snapshot; the chart should
  // call series.update(latest) instead of re-rendering the whole series.
  latest: KLine | null
  connected: boolean
}

// useKLineSeries opens one WS connection to url and tracks both the initial
// historical snapshot and subsequent live updates. Reconnects whenever url
// changes (e.g. the user switches interval, bar type, or param) and on
// disconnect, with a short backoff.
export function useKLineSeries(url: string | null): KLineSeriesUpdate {
  const [history, setHistory] = useState<KLine[]>([])
  const [latest, setLatest] = useState<KLine | null>(null)
  const [connected, setConnected] = useState(false)
  const pendingHistoryRef = useRef<KLine[]>([])

  useEffect(() => {
    if (!url) {
      return
    }

    let cancelled = false
    let ws: WebSocket | null = null
    let retryTimer: ReturnType<typeof setTimeout> | null = null

    const connect = () => {
      if (cancelled) {
        return
      }

      pendingHistoryRef.current = []
      setHistory([])
      setLatest(null)

      ws = new WebSocket(url)

      ws.onopen = () => setConnected(true)

      ws.onmessage = (event) => {
        const msg: WSMessage = JSON.parse(event.data)
        switch (msg.type) {
          case 'snapshot':
            if (msg.data) {
              pendingHistoryRef.current.push(msg.data)
            }
            break
          case 'snapshotEnd':
            setHistory(pendingHistoryRef.current)
            break
          case 'update':
            if (msg.data) {
              setLatest(msg.data)
            }
            break
        }
      }

      ws.onclose = () => {
        setConnected(false)
        if (!cancelled) {
          retryTimer = setTimeout(connect, 2000)
        }
      }

      ws.onerror = () => {
        ws?.close()
      }
    }

    connect()

    return () => {
      cancelled = true
      if (retryTimer) {
        clearTimeout(retryTimer)
      }
      ws?.close()
    }
  }, [url])

  return { history, latest, connected }
}
