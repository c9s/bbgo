import React from 'react';
import CardContent from '@material-ui/core/CardContent';
import Card from '@material-ui/core/Card';
import { makeStyles } from '@material-ui/core/styles';
import List from '@material-ui/core/List';
import ListItem from '@material-ui/core/ListItem';
import ListItemText from '@material-ui/core/ListItemText';
import ListItemAvatar from '@material-ui/core/ListItemAvatar';
import Avatar from '@material-ui/core/Avatar';

const useStyles = makeStyles((theme) => ({
  root: {
    margin: theme.spacing(1),
  },
  cardContent: {},
}));

const logoCurrencies = {
  BTC: true,
  ETH: true,
  BCH: true,
  LTC: true,
  USDT: true,
  BNB: true,
  COMP: true,
  XRP: true,
  LINK: true,
  DOT: true,
  SXP: true,
  DAI: true,
  MAX: true,
  TWD: true,
  SNT: true,
  YFI: true,
  GRT: true,
};

export default function TotalAssetsDetails({ assets }) {
  const classes = useStyles();

  const sortedAssets = [];
  for (let k in assets) {
    sortedAssets.push(assets[k]);
  }
  sortedAssets.sort((a, b) => {
    if (a.inUSD > b.inUSD) {
      return -1;
    }

    if (a.inUSD < b.inUSD) {
      return 1;
    }

    return 0;
  });

  const items = sortedAssets.map((a) => {
    return (
      <ListItem key={a.currency} dense>
        {a.currency in logoCurrencies ? (
          <ListItemAvatar>
            <Avatar
              alt={a.currency}
              src={`/images/${a.currency.toLowerCase()}-logo.svg`}
            />
          </ListItemAvatar>
        ) : (
          <ListItemAvatar>
            <Avatar alt={a.currency} />
          </ListItemAvatar>
        )}
        <ListItemText
          primary={`${a.currency} ${a.total}`}
          secondary={`=~ ${Math.round(a.inUSD)} USD`}
        />
      </ListItem>
    );
  });

  return (
    <Card className={classes.root} variant="outlined">
      <CardContent className={classes.cardContent}>
        <List dense>{items}</List>
      </CardContent>
    </Card>
  );
}
