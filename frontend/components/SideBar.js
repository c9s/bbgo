import Drawer from '@mui/core/Drawer';
import Divider from '@mui/core/Divider';
import List from '@mui/core/List';
import Link from 'next/link';
import ListItem from '@mui/core/ListItem';
import ListItemIcon from '@mui/core/ListItemIcon';
import DashboardIcon from '@mui/icons/Dashboard';
import ListItemText from '@mui/core/ListItemText';
import ListIcon from '@mui/icons/List';
import TrendingUpIcon from '@mui/icons/TrendingUp';
import React from 'react';
import { makeStyles } from '@mui/core/styles';

const drawerWidth = 240;

const useStyles = makeStyles((theme) => ({
  root: {
    flexGrow: 1,
    display: 'flex',
  },
  toolbar: {
    paddingRight: 24, // keep right padding when drawer closed
  },
  toolbarIcon: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    padding: '0 8px',
    ...theme.mixins.toolbar,
  },
  appBarSpacer: theme.mixins.toolbar,
  drawerPaper: {
    [theme.breakpoints.up('sm')]: {
      width: drawerWidth,
      flexShrink: 0,
    },
    position: 'relative',
    whiteSpace: 'nowrap',
    transition: theme.transitions.create('width', {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.enteringScreen,
    }),
  },
  drawer: {
    width: drawerWidth,
  },
}));

export default function SideBar() {
  const classes = useStyles();

  return (
    <Drawer
      variant="permanent"
      className={classes.drawer}
      PaperProps={{
        className: classes.drawerPaper,
      }}
      anchor={'left'}
      open={true}
    >
      <div className={classes.appBarSpacer} />

      <List>
        <Link href={'/'}>
          <ListItem button>
            <ListItemIcon>
              <DashboardIcon />
            </ListItemIcon>
            <ListItemText primary="Dashboard" />
          </ListItem>
        </Link>
      </List>
      <Divider />
      <List>
        <Link href={'/orders'}>
          <ListItem button>
            <ListItemIcon>
              <ListIcon />
            </ListItemIcon>
            <ListItemText primary="Orders" />
          </ListItem>
        </Link>
        <Link href={'/trades'}>
          <ListItem button>
            <ListItemIcon>
              <ListIcon />
            </ListItemIcon>
            <ListItemText primary="Trades" />
          </ListItem>
        </Link>
        <ListItem button>
          <ListItemIcon>
            <TrendingUpIcon />
          </ListItemIcon>
          <ListItemText primary="Strategies" />
        </ListItem>
      </List>
    </Drawer>
  );
}
