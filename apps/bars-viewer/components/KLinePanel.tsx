import React, { useState } from 'react'
import { Select, Group, Paper, Text } from '@mantine/core'
import CandleChart from './CandleChart'
import { useKLineSeries } from '../hooks/useKLineSeries'
import { wsURL } from '../lib/api'

const INTERVALS = ['1m', '5m', '15m', '1h', '4h', '1d']

interface KLinePanelProps {
  symbol: string
}

// KLinePanel is the bottom half: a regular candlestick chart with an interval
// dropdown. Switching the interval reconnects the WS with the new query param.
const KLinePanel = ({ symbol }: KLinePanelProps) => {
  const [interval, setInterval] = useState('1m')

  const url = symbol ? wsURL('/ws/klines', { symbol, interval }) : null
  const { history, latest, connected } = useKLineSeries(url)

  return (
    <Paper shadow="xs" p="sm" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Group position="apart" mb="xs">
        <Group>
          <Select
            label="Interval"
            data={INTERVALS}
            value={interval}
            onChange={(v) => v && setInterval(v)}
            style={{ width: 120 }}
          />
        </Group>
        <Text size="sm" color={connected ? 'teal' : 'red'}>
          {connected ? 'live' : 'disconnected'}
        </Text>
      </Group>
      <div style={{ flex: 1, minHeight: 0 }}>
        <CandleChart history={history} latest={latest} />
      </div>
    </Paper>
  )
}

export default KLinePanel
