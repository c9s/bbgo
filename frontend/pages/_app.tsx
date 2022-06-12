import React from 'react';
import PropTypes from 'prop-types';
import Head from 'next/head';

import { ThemeProvider } from '@mui/material/styles';

import Dialog from '@mui/material/Dialog';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import LinearProgress from '@mui/material/LinearProgress';
import Box from '@mui/material/Box';

import CssBaseline from '@mui/material/CssBaseline';
import theme from '../src/theme';
import '../styles/globals.css';
import { querySessions, querySyncStatus } from '../api/bbgo';

const SyncNotStarted = 0;
const Syncing = 1;
const SyncDone = 2;

// session is configured, check if we're syncing data
let syncStatusPoller = null;

export default function MyApp(props) {
  const { Component, pageProps } = props;

  const [loading, setLoading] = React.useState(true);
  const [syncing, setSyncing] = React.useState(false);

  React.useEffect(() => {
    // Remove the server-side injected CSS.
    const jssStyles = document.querySelector('#jss-server-side');
    if (jssStyles) {
      jssStyles.parentElement.removeChild(jssStyles);
    }

    querySessions((sessions) => {
      if (sessions.length > 0) {
        setSyncing(true);

        const pollSyncStatus = () => {
          querySyncStatus((status) => {
            switch (status) {
              case SyncNotStarted:
                break;
              case Syncing:
                setSyncing(true);
                break;
              case SyncDone:
                clearInterval(syncStatusPoller);
                setLoading(false);
                setSyncing(false);
                break;
            }
          }).catch((err) => {
            console.error(err);
          });
        };

        syncStatusPoller = setInterval(pollSyncStatus, 1000);
      } else {
        // no session found, so we can not sync any data
        setLoading(false);
        setSyncing(false);
      }
    }).catch((err) => {
      console.error(err);
    });
  }, []);

  return (
    <React.Fragment>
      <Head>
        <title>BBGO</title>
        <meta
          name="viewport"
          content="minimum-scale=1, initial-scale=1, width=device-width"
        />
      </Head>
      <ThemeProvider theme={theme}>
        {/* CssBaseline kickstart an elegant, consistent, and simple baseline to build upon. */}
        <CssBaseline />
        {loading ? (
          syncing ? (
            <Dialog
              open={syncing}
              aria-labelledby="alert-dialog-title"
              aria-describedby="alert-dialog-description"
            >
              <DialogTitle id="alert-dialog-title">
                {'Syncing Trades'}
              </DialogTitle>
              <DialogContent>
                <DialogContentText id="alert-dialog-description">
                  The environment is syncing trades from the exchange sessions.
                  Please wait a moment...
                </DialogContentText>
                <Box m={2}>
                  <LinearProgress />
                </Box>
              </DialogContent>
            </Dialog>
          ) : (
            <Dialog
              open={loading}
              aria-labelledby="alert-dialog-title"
              aria-describedby="alert-dialog-description"
            >
              <DialogTitle id="alert-dialog-title">{'Loading'}</DialogTitle>
              <DialogContent>
                <DialogContentText id="alert-dialog-description">
                  Loading...
                </DialogContentText>
                <Box m={2}>
                  <LinearProgress />
                </Box>
              </DialogContent>
            </Dialog>
          )
        ) : (
          <Component {...pageProps} />
        )}
      </ThemeProvider>
    </React.Fragment>
  );
}

MyApp.propTypes = {
  Component: PropTypes.elementType.isRequired,
  pageProps: PropTypes.object.isRequired,
};
