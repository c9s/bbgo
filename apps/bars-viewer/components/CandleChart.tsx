import React, { useEffect, useRef } from 'react'
import { createChart, CrosshairMode, IChartApi, ISeriesApi } from 'lightweight-charts'
import { KLine } from '../lib/types'

const toSeriesPoint = (k: KLine) => ({
  time: (new Date(k.startTime).getTime() / 1000) as any,
  open: k.open,
  high: k.high,
  low: k.low,
  close: k.close,
})

interface CandleChartProps {
  history: KLine[]
  latest: KLine | null
}

// CandleChart renders one candlestick series with lightweight-charts (same
// library apps/backtest-report uses), re-setting the whole series when
// `history` changes (new symbol/interval/bar params) and incrementally
// patching the last candle via `update()` on every live `latest` tick.
const CandleChart = ({ history, latest }: CandleChartProps) => {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const chart = createChart(containerRef.current, {
      width: containerRef.current.clientWidth,
      height: containerRef.current.clientHeight,
      layout: {
        backgroundColor: '#1a1b1e',
        textColor: '#C1C2C5',
      },
      grid: {
        horzLines: { color: '#2C2E33' },
        vertLines: { color: '#2C2E33' },
      },
      timeScale: {
        timeVisible: true,
        borderColor: '#2C2E33',
      },
      rightPriceScale: {
        borderColor: '#2C2E33',
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
    })

    const series = chart.addCandlestickSeries({
      upColor: 'rgb(38,166,154)',
      downColor: 'rgb(255,82,82)',
      wickUpColor: 'rgb(38,166,154)',
      wickDownColor: 'rgb(255,82,82)',
      borderVisible: false,
    })

    chartRef.current = chart
    seriesRef.current = series

    const resizeObserver = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect
      chart.applyOptions({ width, height })
    })
    resizeObserver.observe(containerRef.current)

    return () => {
      resizeObserver.disconnect()
      chart.remove()
      chartRef.current = null
      seriesRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!seriesRef.current || history.length === 0) {
      return
    }
    seriesRef.current.setData(history.map(toSeriesPoint))
    chartRef.current?.timeScale().fitContent()
  }, [history])

  useEffect(() => {
    if (!seriesRef.current || !latest) {
      return
    }
    seriesRef.current.update(toSeriesPoint(latest))
  }, [latest])

  return <div ref={containerRef} style={{ width: '100%', height: '100%' }} />
}

export default CandleChart
