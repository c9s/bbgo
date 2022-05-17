import React, {useEffect, useRef, useState} from 'react';
import {tsvParse} from "d3-dsv";

// https://github.com/tradingview/lightweight-charts/issues/543
// const createChart = dynamic(() => import('lightweight-charts'));
import {createChart} from 'lightweight-charts';

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


const fetchPositionHistory = (setter) => {
  return fetch(
    `/data/bollmaker:ETHUSDT-position.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parsePosition()))
    // .then((data) => tsvParse(data))
    .then((data) => {
      setter(data);
    })
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
};

const fetchOrders = (setter) => {
  return fetch(
    `/data/orders.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parseOrder()))
    // .then((data) => tsvParse(data))
    .then((data) => {
      setter(data);
    })
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
}

const ordersToMarkets = (orders) => {
  const markers = [];
  // var markers = [{ time: data[data.length - 48].time, position: 'aboveBar', color: '#f68410', shape: 'circle', text: 'D' }];
  for (let i = 0; i < orders.length; i++) {
    let order = orders[i];
    switch (order.side) {
      case "BUY":
        markers.push({
          time: order.time.getTime() / 1000.0,
          position: 'belowBar',
          color: '#239D10',
          shape: 'arrowDown',
          // text: 'Buy @ ' + order.price
          text: 'B',
        });
        break;
      case "SELL":
        markers.push({
          time: order.time.getTime() / 1000.0,
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



function fetchKLines(symbol, interval, setter) {
  return fetch(
    `/data/klines/${symbol}-${interval}.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parseKline()))
    // .then((data) => tsvParse(data))
    .then((data) => {
      setter(data);
    })
    .catch((e) => {
      console.error("failed to fetch klines", e)
    });
}

const positionAverageCostHistoryToLineData = (hs) => {
  const avgCosts = [];
  for (let i = 0; i < hs.length; i++) {
    let pos = hs[i];

    if (pos.base == 0) {
      avgCosts.push({
        time: pos.time,
        value: 0,
      });
    } else {
      avgCosts.push({
        time: pos.time,
        value: pos.average_cost,
      });
    }


  }
  return avgCosts;
}

const TradingViewChart = (props) => {
  const ref = useRef();
  const [data, setData] = useState(null);
  const [orders, setOrders] = useState(null);
  const [markers, setMarkers] = useState(null);
  const [positionHistory, setPositionHistory] = useState(null);

  useEffect(() => {
    if (!ref.current || ref.current.children.length > 0) {
      return;
    }

    if (!data || !orders || !markers || !positionHistory) {
      fetchKLines('ETHUSDT', '5m', setData).then(() => {
        fetchOrders((orders) => {
          setOrders(orders);

          const markers = ordersToMarkets(orders);
          setMarkers(markers);
        });
        fetchPositionHistory(setPositionHistory)
      })
      return;
    }

    console.log("createChart")
    const chart = createChart(ref.current, {
      width: 800,
      height: 200,
      timeScale: {
        timeVisible: true,
        borderColor: '#D1D4DC',
      },
      rightPriceScale: {
        borderColor: '#D1D4DC',
      },
      layout: {
        backgroundColor: '#ffffff',
        textColor: '#000',
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

    const series = chart.addCandlestickSeries({
      upColor: 'rgb(38,166,154)',
      downColor: 'rgb(255,82,82)',
      wickUpColor: 'rgb(38,166,154)',
      wickDownColor: 'rgb(255,82,82)',
      borderVisible: false,
    });

    /*
    const data2 = [{ open: 10, high: 10.63, low: 9.49, close: 9.55, time: 1642427876 }, { open: 9.55, high: 10.30, low: 9.42, close: 9.94, time: 1642514276 }, { open: 9.94, high: 10.17, low: 9.92, close: 9.78, time: 1642600676 }, { open: 9.78, high: 10.59, low: 9.18, close: 9.51, time: 1642687076 }, { open: 9.51, high: 10.46, low: 9.10, close: 10.17, time: 1642773476 }, { open: 10.17, high: 10.96, low: 10.16, close: 10.47, time: 1642859876 }, { open: 10.47, high: 11.39, low: 10.40, close: 10.81, time: 1642946276 }, { open: 10.81, high: 11.60, low: 10.30, close: 10.75, time: 1643032676 }, { open: 10.75, high: 11.60, low: 10.49, close: 10.93, time: 1643119076 }, { open: 10.93, high: 11.53, low: 10.76, close: 10.96, time: 1643205476 }];
    series.setData(data2);
    */
    // series.setData(data.slice(0, 95));
    series.setData(data);
    series.setMarkers(markers);

    console.log(positionHistory);
    const lineSeries = chart.addLineSeries();
    lineSeries.setData(positionAverageCostHistoryToLineData(positionHistory));
  }, [ref.current, data])
  return <div>
    <div ref={ref}>

    </div>
  </div>;
};

export default TradingViewChart;
