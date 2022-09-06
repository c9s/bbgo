import { useEffect, useState } from 'react';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Typography from '@mui/material/Typography';
import { makeStyles } from '@mui/styles';

function aggregateAssetsBy(assets, field) {
  let total = 0.0;
  for (let key in assets) {
    if (assets[key]) {
      let a = assets[key];
      let value = a[field];
      total += value;
    }
  }

  return total;
}

const useStyles = makeStyles((theme) => ({
  root: {
    margin: theme.spacing(1),
  },
  title: {
    fontSize: 14,
  },
  pos: {
    marginTop: 12,
  },
}));

export default function TotalAssetSummary({ assets }) {
  const classes = useStyles();
  return (
    <Card className={classes.root} variant="outlined">
      <CardContent>
        <Typography
          className={classes.title}
          color="textSecondary"
          gutterBottom
        >
          Total Account Balance
        </Typography>
        <Typography variant="h5" component="h2">
          {Math.round(aggregateAssetsBy(assets, 'inBTC') * 1e8) / 1e8}{' '}
          <span>BTC</span>
        </Typography>

        <Typography className={classes.pos} color="textSecondary">
          Estimated Value
        </Typography>

        <Typography variant="h5" component="h3">
          {Math.round(aggregateAssetsBy(assets, 'inUSD') * 100) / 100}{' '}
          <span>USD</span>
        </Typography>
      </CardContent>
    </Card>
  );
}
