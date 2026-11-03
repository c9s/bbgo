import React, { useState } from 'react'
import { Select, Group, NumberInput, Paper, Text } from '@mantine/core'
import CandleChart from './CandleChart'
import { useKLineSeries } from '../hooks/useKLineSeries'
import { wsURL } from '../lib/api'
import { BarType } from '../lib/types'

const DEFAULT_PARAM: Record<BarType, number> = {
  tick: 500,
  volume: 10,
}

interface BarsPanelProps {
  symbol: string
}

// BarsPanel is the top half: an indicator/v2 tick-bar or volume-bar chart,
// with a mode dropdown and a numeric param (numTicks or barVolume). Switching
// either reconnects the WS with the new query params.
const BarsPanel = ({ symbol }: BarsPanelProps) => {
  const [barType, setBarType] = useState<BarType>('tick')
  const [param, setParam] = useState<number>(DEFAULT_PARAM.tick)

  const url =
    symbol && param > 0
      ? wsURL('/ws/bars', { symbol, type: barType, param: String(param) })
      : null
  const { history, latest, connected } = useKLineSeries(url)

  return (
    <Paper shadow="xs" p="sm" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Group position="apart" mb="xs">
        <Group>
          <Select
            label="Bar type"
            data={[
              { value: 'tick', label: 'Tick bars' },
              { value: 'volume', label: 'Volume bars' },
            ]}
            value={barType}
            onChange={(v) => {
              if (!v) return
              const next = v as BarType
              setBarType(next)
              setParam(DEFAULT_PARAM[next])
            }}
            style={{ width: 150 }}
          />
          <NumberInput
            label={barType === 'tick' ? 'Ticks per bar' : 'Volume per bar'}
            value={param}
            onChange={(v) => typeof v === 'number' && setParam(v)}
            min={barType === 'tick' ? 1 : 0.0001}
            step={barType === 'tick' ? 100 : 1}
            style={{ width: 150 }}
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

export default BarsPanel
