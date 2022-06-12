import React, { useState } from 'react';
import { useRouter } from 'next/router';

import { makeStyles } from '@mui/styles';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import Grid from '@mui/material/Grid';
import Paper from '@mui/material/Paper';

import TotalAssetsPie from '../components/TotalAssetsPie';
import TotalAssetSummary from '../components/TotalAssetsSummary';
import TotalAssetDetails from '../components/TotalAssetsDetails';

import TradingVolumePanel from '../components/TradingVolumePanel';
import ExchangeSessionTabPanel from '../components/ExchangeSessionTabPanel';

import DashboardLayout from '../layouts/DashboardLayout';

import { queryAssets, querySessions } from '../api/bbgo';

import { ChainId, Config, DAppProvider } from '@usedapp/core';
import { Theme } from '@mui/material/styles';

// fix the `theme.spacing` missing error
// https://stackoverflow.com/a/70707121/3897950
declare module '@mui/styles/defaultTheme' {
  // eslint-disable-next-line @typescript-eslint/no-empty-interface (remove this line if you don't have the rule enabled)
  interface DefaultTheme extends Theme {}
}

const useStyles = makeStyles((theme) => ({
  totalAssetsSummary: {
    margin: theme.spacing(2),
    padding: theme.spacing(2),
  },
  grid: {
    flexGrow: 1,
  },
  control: {
    padding: theme.spacing(2),
  },
}));

const config: Config = {
  readOnlyChainId: ChainId.Mainnet,
  readOnlyUrls: {
    [ChainId.Mainnet]:
      'https://mainnet.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161',
  },
};

// props are pageProps passed from _app.tsx
export default function Home() {
  const classes = useStyles();
  const router = useRouter();

  const [assets, setAssets] = useState({});
  const [sessions, setSessions] = React.useState([]);

  React.useEffect(() => {
    querySessions((sessions) => {
      if (sessions && sessions.length > 0) {
        setSessions(sessions);
        queryAssets(setAssets);
      } else {
        router.push('/setup');
      }
    }).catch((err) => {
      console.error(err);
    });
  }, [router]);

  if (sessions.length == 0) {
    return (
      <DashboardLayout>
        <Box m={4}>
          <Typography variant="h4" gutterBottom>
            Loading
          </Typography>
        </Box>
      </DashboardLayout>
    );
  }

  console.log('index: assets', assets);

  return (
    <DAppProvider config={config}>
      <DashboardLayout>
        <Paper className={classes.totalAssetsSummary}>
          <Typography variant="h4" gutterBottom>
            Total Assets
          </Typography>

          <div className={classes.grid}>
            <Grid
              container
              direction="row"
              justifyContent="space-around"
              alignItems="flex-start"
              spacing={1}
            >
              <Grid item xs={12} md={8}>
                <TotalAssetSummary assets={assets} />
                <TotalAssetsPie assets={assets} />
              </Grid>

              <Grid item xs={12} md={4}>
                <TotalAssetDetails assets={assets} />
              </Grid>
            </Grid>
          </div>
        </Paper>

        <TradingVolumePanel />

        <ExchangeSessionTabPanel />
      </DashboardLayout>
    </DAppProvider>
  );
}
