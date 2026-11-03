import React, { useState } from 'react'
import { Stack } from '@mantine/core'
import SymbolSelect from '../components/SymbolSelect'
import BarsPanel from '../components/BarsPanel'
import KLinePanel from '../components/KLinePanel'

export default function Home() {
  const [symbol, setSymbol] = useState('BTCUSDT')

  return (
    <Stack style={{ height: '100vh', padding: 12 }} spacing="xs">
      <SymbolSelect value={symbol} onChange={setSymbol} />
      <div style={{ flex: 1, minHeight: 0 }}>
        <BarsPanel symbol={symbol} />
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        <KLinePanel symbol={symbol} />
      </div>
    </Stack>
  )
}
