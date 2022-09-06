import { ResponsiveBar } from '@nivo/bar';
import { queryTradingVolume } from '../api/bbgo';
import { useEffect, useState } from 'react';

function toPeriodDateString(time, period) {
  switch (period) {
    case 'day':
      return (
        time.getFullYear() + '-' + (time.getMonth() + 1) + '-' + time.getDate()
      );
    case 'month':
      return time.getFullYear() + '-' + (time.getMonth() + 1);
    case 'year':
      return time.getFullYear();
  }

  return (
    time.getFullYear() + '-' + (time.getMonth() + 1) + '-' + time.getDate()
  );
}

function groupData(rows, period, segment) {
  let dateIndex = {};
  let startTime = null;
  let endTime = null;
  let keys = {};

  rows.forEach((v) => {
    const time = new Date(v.time);
    if (!startTime) {
      startTime = time;
    }

    endTime = time;

    const dateStr = toPeriodDateString(time, period);
    const key = v[segment];

    keys[key] = true;

    const k = key ? key : 'total';
    const quoteVolume = Math.round(v.quoteVolume * 100) / 100;

    if (dateIndex[dateStr]) {
      dateIndex[dateStr][k] = quoteVolume;
    } else {
      dateIndex[dateStr] = {
        date: dateStr,
        year: time.getFullYear(),
        month: time.getMonth() + 1,
        day: time.getDate(),
        [k]: quoteVolume,
      };
    }
  });

  let data = [];
  while (startTime < endTime) {
    const dateStr = toPeriodDateString(startTime, period);
    const groupData = dateIndex[dateStr];
    if (groupData) {
      data.push(groupData);
    } else {
      data.push({
        date: dateStr,
        year: startTime.getFullYear(),
        month: startTime.getMonth() + 1,
        day: startTime.getDate(),
        total: 0,
      });
    }

    switch (period) {
      case 'day':
        startTime.setDate(startTime.getDate() + 1);
        break;
      case 'month':
        startTime.setMonth(startTime.getMonth() + 1);
        break;
      case 'year':
        startTime.setFullYear(startTime.getFullYear() + 1);
        break;
    }
  }

  return [data, Object.keys(keys)];
}

export default function TradingVolumeBar(props) {
  const [tradingVolumes, setTradingVolumes] = useState([]);
  const [period, setPeriod] = useState(props.period);
  const [segment, setSegment] = useState(props.segment);

  useEffect(() => {
    if (props.period !== period) {
      setPeriod(props.period);
    }

    if (props.segment !== segment) {
      setSegment(props.segment);
    }

    queryTradingVolume(
      { period: props.period, segment: props.segment },
      (tradingVolumes) => {
        setTradingVolumes(tradingVolumes);
      }
    );
  }, [props.period, props.segment]);

  const [data, keys] = groupData(tradingVolumes, period, segment);

  return (
    <ResponsiveBar
      keys={keys}
      data={data}
      indexBy={'date'}
      margin={{ top: 50, right: 160, bottom: 100, left: 60 }}
      padding={0.3}
      valueScale={{ type: 'linear' }}
      indexScale={{ type: 'band', round: true }}
      labelSkipWidth={30}
      labelSkipHeight={20}
      enableGridY={true}
      colors={{ scheme: 'paired' }}
      axisBottom={{
        tickRotation: -90,
        legend: period,
        legendPosition: 'middle',
        legendOffset: 80,
      }}
      legends={[
        {
          dataFrom: 'keys',
          anchor: 'right',
          direction: 'column',
          justify: false,
          translateX: 120,
          translateY: 0,
          itemsSpacing: 2,
          itemWidth: 100,
          itemHeight: 20,
          itemDirection: 'left-to-right',
          itemOpacity: 0.85,
          symbolSize: 20,
          effects: [
            {
              on: 'hover',
              style: {
                itemOpacity: 1,
              },
            },
          ],
        },
      ]}
      animate={true}
      motionStiffness={90}
      motionDamping={15}
    />
  );
}
