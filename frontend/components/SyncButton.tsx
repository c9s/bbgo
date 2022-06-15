import { styled } from '@mui/styles';
import React, { useEffect, useState } from 'react';
import { querySyncStatus, SyncStatus, triggerSync } from '../api/bbgo';
import useInterval from '../hooks/useInterval';

const ToolbarButton = styled('button')(({ theme }) => ({
  padding: theme.spacing(1),
}));

export default function SyncButton() {
  const [syncing, setSyncing] = useState(false);

  const sync = async () => {
    try {
      setSyncing(true);
      await triggerSync();
    } catch {
      setSyncing(false);
    }
  };

  useEffect(() => {
    sync();
  }, []);

  useInterval(() => {
    querySyncStatus().then((s) => {
      if (s !== SyncStatus.Syncing) {
        setSyncing(false);
      }
    });
  }, 2000);

  return (
    <ToolbarButton disabled={syncing} onClick={sync}>
      {syncing ? 'Syncing...' : 'Sync'}
    </ToolbarButton>
  );
}
