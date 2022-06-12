import React, { useEffect, useState } from 'react';

import { ResponsivePie } from '@nivo/pie';
import { queryAssets } from '../api/bbgo';
import { currencyColor } from '../src/utils';
import CardContent from '@mui/material/CardContent';
import Card from '@mui/material/Card';
import { makeStyles } from '@mui/styles';

function reduceAssetsBy(assets, field, minimum) {
  let as = [];

  let others = { id: 'others', labels: 'others', value: 0.0 };
  for (let key in assets) {
    if (assets[key]) {
      let a = assets[key];
      let value = a[field];

      if (value < minimum) {
        others.value += value;
      } else {
        as.push({
          id: a.currency,
          label: a.currency,
          color: currencyColor(a.currency),
          value: Math.round(value, 1),
        });
      }
    }
  }

  return as;
}

const useStyles = makeStyles((theme) => ({
  root: {
    margin: theme.spacing(1),
  },
  cardContent: {
    height: 350,
  },
}));

export default function TotalAssetsPie({ assets }) {
  const classes = useStyles();
  return (
    <Card className={classes.root} variant="outlined">
      <CardContent className={classes.cardContent}>
        <ResponsivePie
          data={reduceAssetsBy(assets, 'inUSD', 2)}
          margin={{ top: 20, right: 80, bottom: 10, left: 0 }}
          padding={0.1}
          innerRadius={0.8}
          padAngle={1.0}
          valueFormat=" >-$f"
          colors={{ datum: 'data.color' }}
          // colors={{scheme: 'nivo'}}
          cornerRadius={0.1}
          borderWidth={1}
          borderColor={{ from: 'color', modifiers: [['darker', 0.2]] }}
          radialLabelsSkipAngle={10}
          radialLabelsTextColor="#333333"
          radialLabelsLinkColor={{ from: 'color' }}
          sliceLabelsSkipAngle={30}
          sliceLabelsTextColor="#fff"
          legends={[
            {
              anchor: 'right',
              direction: 'column',
              justify: false,
              translateX: 70,
              translateY: 0,
              itemsSpacing: 5,
              itemWidth: 80,
              itemHeight: 24,
              itemTextColor: '#999',
              itemOpacity: 1,
              symbolSize: 18,
              symbolShape: 'circle',
              effects: [
                {
                  on: 'hover',
                  style: {
                    itemTextColor: '#000',
                  },
                },
              ],
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
