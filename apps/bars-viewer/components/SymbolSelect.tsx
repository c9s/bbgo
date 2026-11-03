import React, { useEffect, useState } from 'react'
import { Select } from '@mantine/core'
import { API_BASE_URL } from '../lib/api'

interface SymbolSelectProps {
  value: string
  onChange: (symbol: string) => void
}

// SymbolSelect loads the backend's configured symbol list once and lets the
// user pick one; both KLinePanel and BarsPanel share the selected symbol.
const SymbolSelect = ({ value, onChange }: SymbolSelectProps) => {
  const [symbols, setSymbols] = useState<string[]>([])

  useEffect(() => {
    fetch(`${API_BASE_URL}/api/symbols`)
      .then((res) => res.json())
      .then((data: string[]) => setSymbols(data))
      .catch((e) => console.error('failed to load symbols', e))
  }, [])

  return (
    <Select
      label="Symbol"
      data={symbols}
      value={value}
      onChange={(v) => v && onChange(v)}
      style={{ width: 160 }}
    />
  )
}

export default SymbolSelect
