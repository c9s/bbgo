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
    d.time = d.startTime;

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


const TradingViewChart = (props) => {
  const ref = useRef();
  const [data, setData] = useState(null);

  useEffect(() => {
    console.log("useEffect")

    setData(true)

    if (!ref.current || ref.current.children.length > 0) {
      return;
    }

    fetch(
      `/data/klines/ETHUSDT-5m.tsv`,
    )
      .then((response) => response.text())
      .then((data) => tsvParse(data, parseKline()))
      // .then((data) => tsvParse(data))
      .then((data) => {
        setData(data);
      })
      .catch(() => {
        console.error("failed to fetch")
      });


    // ref.current

    console.log("createChart")
    const c = createChart(ref.current, {
      width: 800,
      height: 200,
    });

    const lineSeries = c.addLineSeries();
    lineSeries.setData([
      {time: '2019-04-11', value: 80.01},
      {time: '2019-04-12', value: 96.63},
      {time: '2019-04-13', value: 76.64},
      {time: '2019-04-14', value: 81.89},
      {time: '2019-04-15', value: 74.43},
      {time: '2019-04-16', value: 80.01},
      {time: '2019-04-17', value: 96.63},
      {time: '2019-04-18', value: 76.64},
      {time: '2019-04-19', value: 81.89},
      {time: '2019-04-20', value: 74.43},
    ]);
  }, [ref.current, data])
  return <div>
    <div ref={ref}>

    </div>
  </div>;
};

export default TradingViewChart;
