import React, {useEffect, useState, useRef} from 'react';
import dynamic from 'next/dynamic';
import { tsvParse } from "d3-dsv";

// https://github.com/tradingview/lightweight-charts/issues/543
// const createChart = dynamic(() => import('lightweight-charts'));

import { createChart } from 'lightweight-charts';
import {timeParse} from "d3-time-format";

const parseDate = timeParse("%Y-%m-%d");

const parseData = () => {
  return (d) => {
    const date = parseDate(d.startTime);
    if (date === null) {
      d.time = new Date(Number(d.startTime));
    } else {
      d.time = new Date(date);
    }

    for (const key in d) {
      // convert number fields
      if (key !== "time" && key !== "interval" && Object.prototype.hasOwnProperty.call(d, key)) {
        d[key] = +d[key];
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
      .then((data) => tsvParse(data, parseData()))
      // .then((data) => tsvParse(data))
      .then((data) => {
        console.log(data);
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
      { time: '2019-04-11', value: 80.01 },
      { time: '2019-04-12', value: 96.63 },
      { time: '2019-04-13', value: 76.64 },
      { time: '2019-04-14', value: 81.89 },
      { time: '2019-04-15', value: 74.43 },
      { time: '2019-04-16', value: 80.01 },
      { time: '2019-04-17', value: 96.63 },
      { time: '2019-04-18', value: 76.64 },
      { time: '2019-04-19', value: 81.89 },
      { time: '2019-04-20', value: 74.43 },
    ]);
  }, [ref.current, data])
  return <div>
    <div ref={ref}>

    </div>
  </div>;
};

export default TradingViewChart;
