import React, {useEffect, useRef, useState} from 'react';
import {tsvParse} from "d3-dsv";

// https://github.com/tradingview/lightweight-charts/issues/543
// const createChart = dynamic(() => import('lightweight-charts'));
import {createChart, CrosshairMode} from 'lightweight-charts';
import {Button} from "@nextui-org/react";

// const parseDate = timeParse("%Y-%m-%d");

const parseKline = () => {
  return (d) => {
    d.startTime = new Date(Number(d.startTime) * 1000);
    d.endTime = new Date(Number(d.endTime) * 1000);
    d.time = d.startTime.getTime() / 1000;

    for (const key in d) {
      // convert number fields
      if (Object.prototype.hasOwnProperty.call(d, key)) {
        switch (key) {
          case "open":
          case "high":
          case "low":
          case "close":
          case "volume":
            d[key] = +d[key];
            break
        }
      }
    }

    return d;
  };
};


const parseOrder = () => {
  return (d) => {
    for (const key in d) {
      // convert number fields
      if (Object.prototype.hasOwnProperty.call(d, key)) {
        switch (key) {
          case "order_id":
          case "price":
          case "quantity":
            d[key] = +d[key];
            break;
          case "time":
            d[key] = new Date(d[key]);
            break;
        }
      }
    }
    return d;
  };
}

const parsePosition = () => {
  return (d) => {
    for (const key in d) {
      // convert number fields
      if (Object.prototype.hasOwnProperty.call(d, key)) {
        switch (key) {
          case "accumulated_profit":
          case "average_cost":
          case "quote":
          case "base":
            d[key] = +d[key];
            break
          case "time":
            d[key] = new Date(d[key]);
            break
        }
      }
    }
    return d;
  };
}

const fetchPositionHistory = (basePath, runID, setter) => {
  // TODO: load the filename from the manifest
  return fetch(
    `${basePath}/${runID}/bollmaker:ETHUSDT-position.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parsePosition()))
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
};

const fetchOrders = (basePath, runID, setter) => {
  return fetch(
    `${basePath}/${runID}/orders.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parseOrder()))
    .then((data) => {
      setter(data);
    })
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
}

const parseInterval = (s) => {
  switch (s) {
    case "1m":
      return 60;
    case "5m":
      return 60 * 5;
    case "15m":
      return 60 * 15;
    case "30m":
      return 60 * 30;
    case "1h":
      return 60 * 60;
    case "4h":
      return 60 * 60 * 4;
    case "6h":
      return 60 * 60 * 6;
    case "12h":
      return 60 * 60 * 12;
    case "1d":
      return 60 * 60 * 24;
  }

  return 60;
};

const orderAbbr = (order) => {
  let s = '';
  switch (order.side) {
    case "BUY":
      s += 'B';
      break;
    case "SELL":
      s += 'S';
      break
  }

  switch (order.order_type) {
    case "STOP_LIMIT":
      s += ' StopLoss';
  }
  return s
}

const ordersToMarkets = (interval, orders) => {
  const markers = [];
  const intervalSecs = parseInterval(interval);

  // var markers = [{ time: data[data.length - 48].time, position: 'aboveBar', color: '#f68410', shape: 'circle', text: 'D' }];
  for (let i = 0; i < orders.length; i++) {
    let order = orders[i];
    let t = order.time.getTime() / 1000.0;
    let lastMarker = markers.length > 0 ? markers[markers.length - 1] : null;
    if (lastMarker) {
      let remainder = lastMarker.time % intervalSecs;
      let startTime = lastMarker.time - remainder;
      let endTime = (startTime + intervalSecs);
      // skip the marker in the same interval of the last marker
      if (t < endTime) {
        continue
      }
    }

    switch (order.side) {
      case "BUY":
        markers.push({
          time: t,
          position: 'belowBar',
          color: '#239D10',
          shape: 'arrowDown',
          // text: 'Buy @ ' + order.price
          text: 'B',
        });
        break;
      case "SELL":
        markers.push({
          time: t,
          position: 'aboveBar',
          color: '#e91e63',
          shape: 'arrowDown',
          // text: 'Sell @ ' + order.price
          text: 'S',
        });
        break;
    }
  }
  return markers;
};

const removeDuplicatedKLines = (klines) => {
  const newK = [];
  for (let i = 0; i < klines.length; i++) {
    const k = klines[i];

    if (i > 0 && k.time === klines[i - 1].time) {
      continue
    }

    newK.push(k);
  }
  return newK;
}

function fetchKLines(basePath, runID, symbol, interval, setter) {
  return fetch(
    `${basePath}/${runID}/klines/${symbol}-${interval}.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parseKline()))
    // .then((data) => tsvParse(data))
    .catch((e) => {
      console.error("failed to fetch klines", e)
    });
}

const klinesToVolumeData = (klines) => {
  const volumes = [];

  for (let i = 0; i < klines.length; i++) {
    const kline = klines[i];
    volumes.push({
      time: (kline.startTime.getTime() / 1000),
      value: kline.volume,
    })
  }

  return volumes;
}


const positionBaseHistoryToLineData = (interval, hs) => {
  const bases = [];
  const intervalSeconds = parseInterval(interval);
  for (let i = 0; i < hs.length; i++) {
    const pos = hs[i];
    let t = pos.time.getTime() / 1000;
    t = (t - t % intervalSeconds)

    if (i > 0 && (pos.base === hs[i - 1].base || t === hs[i - 1].time)) {
      continue;
    }

    bases.push({
      time: t,
      value: pos.base,
    });
  }
  return bases;
}


const positionAverageCostHistoryToLineData = (interval, hs) => {
  const avgCosts = [];
  const intervalSeconds = parseInterval(interval);
  for (let i = 0; i < hs.length; i++) {
    const pos = hs[i];
    let t = pos.time.getTime() / 1000;
    t = (t - t % intervalSeconds)

    if (i > 0 && (pos.average_cost === hs[i - 1].average_cost || t === hs[i - 1].time)) {
      continue;
    }

    if (pos.base === 0) {
      avgCosts.push({
        time: t,
        value: 0,
      });
    } else {
      avgCosts.push({
        time: t,
        value: pos.average_cost,
      });
    }


  }
  return avgCosts;
}

const TradingViewChart = (props) => {
  const chartContainerRef = useRef();
  const chart = useRef();
  const resizeObserver = useRef();
  const [data, setData] = useState(null);
  const [orders, setOrders] = useState(null);
  const [markers, setMarkers] = useState(null);
  const [positionHistory, setPositionHistory] = useState(null);
  const [currentInterval, setCurrentInterval] = useState('5m');

  const intervals = props.intervals || [];

  useEffect(() => {
    if (!chartContainerRef.current || chartContainerRef.current.children.length > 0) {
      return;
    }

    if (!data) {
      const ordersFetcher = fetchOrders(props.basePath, props.runID, (orders) => {
        const markers = ordersToMarkets(currentInterval, orders);
        setOrders(orders);
        setMarkers(markers);
      });

      const positionHistoryFetcher = fetchPositionHistory(props.basePath, props.runID).then((data) => {
        setPositionHistory(data);
      });

      Promise.all([ordersFetcher, positionHistoryFetcher]).then(() => {
        fetchKLines(props.basePath, props.runID, 'ETHUSDT', currentInterval).then((data) => {
          setData(removeDuplicatedKLines(data));
        })
      });

      return;
    }

    console.log("createChart")

    chart.current = createChart(chartContainerRef.current, {
      width: chartContainerRef.current.clientWidth,
      height: chartContainerRef.current.clientHeight,
      timeScale: {
        timeVisible: true,
        borderColor: '#D1D4DC',
      },
      rightPriceScale: {
        borderColor: '#D1D4DC',
      },
      leftPriceScale: {
        visible: true,
        borderColor: 'rgba(197, 203, 206, 1)',
      },
      layout: {
        backgroundColor: '#ffffff',
        textColor: '#000',
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
      grid: {
        horzLines: {
          color: '#F0F3FA',
        },
        vertLines: {
          color: '#F0F3FA',
        },
      },
    });

    const series = chart.current.addCandlestickSeries({
      upColor: 'rgb(38,166,154)',
      downColor: 'rgb(255,82,82)',
      wickUpColor: 'rgb(38,166,154)',
      wickDownColor: 'rgb(255,82,82)',
      borderVisible: false,
    });
    series.setData(data);
    series.setMarkers(markers);

    const lineSeries = chart.current.addLineSeries();
    const costLine = positionAverageCostHistoryToLineData(currentInterval, positionHistory);
    lineSeries.setData(costLine);

    const baseLineSeries = chart.current.addLineSeries({
      priceScaleId: 'left',
      color: '#98338C',
    });
    const baseLine = positionBaseHistoryToLineData(currentInterval, positionHistory)
    baseLineSeries.setData(baseLine);


    const volumeData = klinesToVolumeData(data);
    const volumeSeries = chart.current.addHistogramSeries({
      color: '#182233',
      lineWidth: 2,
      priceFormat: {
        type: 'volume',
      },
      overlay: true,
      scaleMargins: {
        top: 0.8,
        bottom: 0,
      },
    });
    volumeSeries.setData(volumeData);

    chart.current.timeScale().fitContent();
    return () => {
      chart.current.remove();
      setData(null);
    };
  }, [props.runID, currentInterval, data])

  // see:
  // https://codesandbox.io/s/9inkb?file=/src/styles.css
  useEffect(() => {
    resizeObserver.current = new ResizeObserver(entries => {
      if (!chart.current) {
        return;
      }

      const {width, height} = entries[0].contentRect;
      chart.current.applyOptions({width, height});

      setTimeout(() => {
        chart.current.timeScale().fitContent();
      }, 0);
    });

    resizeObserver.current.observe(chartContainerRef.current);
    return () => resizeObserver.current.disconnect();
  }, []);

  return (
    <div>
      <Button.Group>
        {intervals.map((interval) => {
          return <Button size="xs" key={interval} onPress={(e) => {
            setCurrentInterval(interval)
          }}>
            {interval}
          </Button>
        })}
      </Button.Group>
      <div ref={chartContainerRef} style={{'flex': 1, 'minHeight': 300}}>
      </div>
    </div>
  );
};

export default TradingViewChart;
