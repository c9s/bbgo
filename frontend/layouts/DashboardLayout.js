import React from 'react';

import { makeStyles } from '@mui/styles';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import Container from '@mui/material/Container';

import SideBar from '../components/SideBar';
import SyncButton from '../components/SyncButton';

import ConnectWallet from '../components/ConnectWallet';
import { Box } from '@mui/material';

const useStyles = makeStyles((theme) => ({
  root: {
    flexGrow: 1,
    display: 'flex',
  },
  content: {
    flexGrow: 1,
    height: '100vh',
    overflow: 'auto',
  },
  appBar: {
    zIndex: theme.zIndex.drawer + 1,
  },
  appBarSpacer: theme.mixins.toolbar,
  container: {},
  toolbar: {
    justifyContent: 'space-between',
  },
}));

export default function DashboardLayout({ children }) {
  const classes = useStyles();

  return (
    <div className={classes.root}>
      <AppBar className={classes.appBar}>
        <Toolbar className={classes.toolbar}>
          <Typography variant="h6" className={classes.title}>
            BBGO
          </Typography>
          <Box sx={{ flexGrow: 1 }} />
          <SyncButton />
          <ConnectWallet />
        </Toolbar>
      </AppBar>

      <SideBar />

      <main className={classes.content}>
        <div className={classes.appBarSpacer} />
        <Container
          className={classes.container}
          maxWidth={false}
          disableGutters={true}
        >
          {children}
        </Container>
      </main>
    </div>
  );
}
