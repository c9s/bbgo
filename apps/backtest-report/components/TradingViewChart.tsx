import React, {useEffect, useRef, useState} from 'react';
import {tsvParse} from "d3-dsv";
import {Checkbox, Group, SegmentedControl} from '@mantine/core';

// https://github.com/tradingview/lightweight-charts/issues/543
// const createChart = dynamic(() => import('lightweight-charts'));
import {createChart, CrosshairMode, MouseEventParams, TimeRange} from 'lightweight-charts';
import {Order, ReportSummary} from "../types";
import moment from "moment";
import {format} from 'date-fns';

import OrderListTable from './OrderListTable';

// See https://codesandbox.io/s/ve7w2?file=/src/App.js
import TimeRangeSlider from './TimeRangeSlider';

const parseKline = () => {
  return (d: any) => {
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

const selectKLines = (klines: KLine[], startTime: Date, endTime: Date): KLine[] => {
  const selected = [];
  for (let i = 0; i < klines.length; i++) {
    const k = klines[i]
    if (k.startTime < startTime) {
      continue
    }
    if (k.startTime > endTime) {
      break
    }

    selected.push(k)
  }

  return selected
}


const parseOrder = () => {
  return (d: any) => {
    for (const key in d) {
      // convert number fields
      if (Object.prototype.hasOwnProperty.call(d, key)) {
        switch (key) {
          case "order_id":
          case "price":
          case "quantity":
            d[key] = +d[key];
            break;
          case "update_time":
          case "creation_time":
          case "time":
            d[key] = moment(d[key], 'dddd, DD MMM YYYY h:mm:ss').toDate();
            break;
        }
      }
    }
    return d;
  };
}

const parsePosition = () => {
  return (d: any) => {
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

const fetchPositionHistory = (basePath: string, runID: string, filename: string) => {
  return fetch(
    `${basePath}/${runID}/${filename}`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parsePosition()) as Array<PositionHistoryEntry>)
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
};

const selectPositionHistory = (data: PositionHistoryEntry[], since: Date, until: Date): PositionHistoryEntry[] => {
  const entries: PositionHistoryEntry[] = [];
  for (let i = 0; i < data.length; i++) {
    const d = data[i];
    if (d.time < since || d.time > until) {
      continue
    }

    entries.push(d)
  }
  return entries
}

const fetchOrders = (basePath: string, runID: string) => {
  return fetch(
    `${basePath}/${runID}/orders.tsv`,
  )
    .then((response) => response.text())
    .then((data: string) => tsvParse(data, parseOrder()) as Array<Order>)
    .catch((e) => {
      console.error("failed to fetch orders", e)
    });
}

const selectOrders = (data: Order[], since: Date, until: Date): Order[] => {
  const entries: Order[] = [];
  for (let i = 0; i < data.length; i++) {
    const d = data[i];
    if (d.time && (d.time < since || d.time > until)) {
      continue
    }

    entries.push(d);
  }
  return entries
}

const parseInterval = (s: string) => {
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


interface Marker {
  time: number;
  position: string;
  color: string;
  shape: string;
  text: string;
}

const ordersToMarkers = (interval: string, orders: Array<Order> | void): Array<Marker> => {
  const markers: Array<Marker> = [];
  const intervalSecs = parseInterval(interval);

  if (!orders) {
    return markers;
  }

  // var markers = [{ time: data[data.length - 48].time, position: 'aboveBar', color: '#f68410', shape: 'circle', text: 'D' }];
  for (let i = 0; i < orders.length; i++) {
    let order = orders[i];
    let t = (order.update_time || order.time).getTime() / 1000.0;
    let lastMarker = markers.length > 0 ? markers[markers.length - 1] : null;
    if (lastMarker) {
      let remainder = lastMarker.time % intervalSecs;
      let startTime = lastMarker.time - remainder;
      let endTime = (startTime + intervalSecs);
      // skip the marker in the same interval of the last marker
      if (t < endTime) {
        // continue
      }
    }

    let text = '' + order.price
    if (order.tag) {
      text += " #" + order.tag;
    }

    switch (order.side) {
      case "BUY":
        markers.push({
          time: t,
          position: 'belowBar',
          color: '#239D10',
          shape: 'arrowUp',
          text: text,
        });
        break;
      case "SELL":
        markers.push({
          time: t,
          position: 'aboveBar',
          color: '#e91e63',
          shape: 'arrowDown',
          text: text,
        });
        break;
    }
  }
  return markers;
};

const removeDuplicatedKLines = (klines: Array<KLine>): Array<KLine> => {
  const newK = [];
  for (let i = 0; i < klines.length; i++) {
    const k = klines[i];

    if (i > 0 && k.time === klines[i - 1].time) {
      console.warn(`duplicated kline at index ${i}`, k)
      continue
    }

    newK.push(k);
  }
  return newK;
}

function fetchKLines(basePath: string, runID: string, symbol: string, interval: string, startTime: Date, endTime: Date) {
  var duration = [moment(startTime).format('YYYYMMDD'), moment(endTime).format('YYYYMMDD')];
  return fetch(
    `${basePath}/shared/klines_${duration.join('-')}/${symbol}-${interval}.tsv`,
  )
    .then((response) => response.text())
    .then((data) => tsvParse(data, parseKline()))
    .catch((e) => {
      console.error("failed to fetch klines", e)
    });
}

interface KLine {
  time: Date;
  startTime: Date;
  endTime: Date;
  interval: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

const klinesToVolumeData = (klines: Array<KLine>) => {
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


interface PositionHistoryEntry {
  time: Date;
  base: number;
  quote: number;
  average_cost: number;
}

const positionBaseHistoryToLineData = (interval: string, hs: Array<PositionHistoryEntry>) => {
  const bases = [];
  const intervalSeconds = parseInterval(interval);
  for (let i = 0; i < hs.length; i++) {
    const pos = hs[i];
    if (!pos.time) {
      console.warn('position history record missing time field', pos)
      continue
    }

    // ignore duplicated entry
    if (i > 0 && hs[i].time.getTime() === hs[i - 1].time.getTime()) {
      continue
    }

    let t = pos.time.getTime() / 1000;
    t = (t - t % intervalSeconds)

    if (i > 0 && (pos.base === hs[i - 1].base)) {
      continue;
    }

    bases.push({
      time: t,
      value: pos.base,
    });
  }
  return bases;
}


const positionAverageCostHistoryToLineData = (interval: string, hs: Array<PositionHistoryEntry>) => {
  const avgCosts = [];
  const intervalSeconds = parseInterval(interval);
  for (let i = 0; i < hs.length; i++) {
    const pos = hs[i];

    if (!pos.time) {
      console.warn('position history record missing time field', pos)
      continue
    }

    // ignore duplicated entry
    if (i > 0 && hs[i].time.getTime() === hs[i - 1].time.getTime()) {
      continue
    }


    let t = pos.time.getTime() / 1000;
    t = (t - t % intervalSeconds)

    if (i > 0 && (pos.average_cost === hs[i - 1].average_cost)) {
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

const createBaseChart = (chartContainerRef: React.RefObject<any>) => {
  return createChart(chartContainerRef.current, {
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
};


interface TradingViewChartProps {
  basePath: string;
  runID: string;
  reportSummary: ReportSummary;
  symbol: string;
}

const TradingViewChart = (props: TradingViewChartProps) => {
  const chartContainerRef = useRef<any>();
  const chart = useRef<any>();
  const resizeObserver = useRef<any>();
  const intervals = props.reportSummary.intervals || [];

  intervals.sort((a, b) => {
    const as = parseInterval(a)
    const bs = parseInterval(b)
    if (as < bs) {
      return -1;
    } else if (as > bs) {
      return 1;
    }
    return 0;
  })

  const [currentInterval, setCurrentInterval] = useState(intervals.length > 0 ? intervals[intervals.length - 1] : '1m');
  const [showPositionBase, setShowPositionBase] = useState(false);
  const [showPositionAverageCost, setShowPositionAverageCost] = useState(false);
  const [orders, setOrders] = useState<Order[]>([]);

  const reportTimeRange = [
    new Date(props.reportSummary.startTime),
    new Date(props.reportSummary.endTime),
  ]
  const [selectedTimeRange, setSelectedTimeRange] = useState(reportTimeRange)

  useEffect(() => {
    if (!chartContainerRef.current || chartContainerRef.current.children.length > 0) {
      return;
    }

    const chartData: any = {};
    const fetchers = [];
    const ordersFetcher = fetchOrders(props.basePath, props.runID).then((orders: Order[] | void) => {
      if (orders) {
        const markers = ordersToMarkers(currentInterval, selectOrders(orders, selectedTimeRange[0], selectedTimeRange[1]));
        chartData.orders = orders;
        chartData.markers = markers;
        setOrders(orders);
      }
      return orders;
    });
    fetchers.push(ordersFetcher);

    if (props.reportSummary && props.reportSummary.manifests && props.reportSummary.manifests.length === 1) {
      const manifest = props.reportSummary?.manifests[0];
      if (manifest && manifest.type === "strategyProperty" && manifest.strategyProperty === "position") {
        const positionHistoryFetcher = fetchPositionHistory(props.basePath, props.runID, manifest.filename).then((data) => {
          chartData.positionHistory = selectPositionHistory(data as PositionHistoryEntry[], selectedTimeRange[0], selectedTimeRange[1]);
          // chartData.positionHistory = data;
        });
        fetchers.push(positionHistoryFetcher);
      }
    }

    const kLinesFetcher = fetchKLines(props.basePath, props.runID, props.symbol, currentInterval, new Date(props.reportSummary.startTime), new Date(props.reportSummary.endTime)).then((klines) => {
      if (klines) {
        chartData.allKLines = removeDuplicatedKLines(klines)
        chartData.klines = selectKLines(chartData.allKLines, selectedTimeRange[0], selectedTimeRange[1])
      }
    });
    fetchers.push(kLinesFetcher);

    Promise.all(fetchers).then(() => {
      console.log("createChart")
      chart.current = createBaseChart(chartContainerRef);

      const series = chart.current.addCandlestickSeries({
        upColor: 'rgb(38,166,154)',
        downColor: 'rgb(255,82,82)',
        wickUpColor: 'rgb(38,166,154)',
        wickDownColor: 'rgb(255,82,82)',
        borderVisible: false,
      });
      series.setData(chartData.klines);
      series.setMarkers(chartData.markers);

      const ohlcLegend = createLegend(0, 'rgba(0, 0, 0, 1)');
      chartContainerRef.current.appendChild(ohlcLegend);

      const updateOHLCLegend = createOHLCLegendUpdater(ohlcLegend, "")
      chart.current.subscribeCrosshairMove((param: MouseEventParams) => {
        updateOHLCLegend(param.seriesPrices.get(series), param.time);
      });

      [9, 27, 99].forEach((w, i) => {
        const emaValues = calculateEMA(chartData.klines, w)
        const emaColor = 'rgba(' + w + ', ' + (111 - w) + ', 232, 0.9)'
        const emaLine = chart.current.addLineSeries({
          color: emaColor,
          lineWidth: 1,
          lastValueVisible: false,
        });
        emaLine.setData(emaValues);

        const legend = createLegend(i + 1, emaColor)
        chartContainerRef.current.appendChild(legend);

        const updateLegendText = createLegendUpdater(legend, 'EMA ' + w)

        updateLegendText(emaValues[emaValues.length - 1].value);
        chart.current.subscribeCrosshairMove((param: MouseEventParams) => {
          updateLegendText(param.seriesPrices.get(emaLine));
        });
      })

      const volumeData = klinesToVolumeData(chartData.klines);
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

      if (chartData.positionHistory) {
        if (showPositionAverageCost) {
          const costLineSeries = chart.current.addLineSeries();
          const costLine = positionAverageCostHistoryToLineData(currentInterval, chartData.positionHistory);
          costLineSeries.setData(costLine);
        }

        if (showPositionBase) {
          const baseLineSeries = chart.current.addLineSeries({
            priceScaleId: 'left',
            color: '#98338C',
          });
          const baseLine = positionBaseHistoryToLineData(currentInterval, chartData.positionHistory)
          baseLineSeries.setData(baseLine);
        }
      }

      chart.current.timeScale().fitContent();

      /*
      chart.current.timeScale().setVisibleRange({
        from: (new Date(Date.UTC(2018, 0, 1, 0, 0, 0, 0))).getTime() / 1000,
        to: (new Date(Date.UTC(2018, 1, 1, 0, 0, 0, 0))).getTime() / 1000,
      });
      */

      // see:
      // https://codesandbox.io/s/9inkb?file=/src/styles.css
      resizeObserver.current = new ResizeObserver(entries => {
        if (!chart.current) {
          return;
        }

        const {width, height} = entries[0].contentRect;
        chart.current.applyOptions({width, height});

        setTimeout(() => {
          if (chart.current) {
            chart.current.timeScale().fitContent();
          }
        }, 0);
      });

      resizeObserver.current.observe(chartContainerRef.current);
    });

    return () => {
      console.log("removeChart")

      if (resizeObserver.current) {
        resizeObserver.current.disconnect();
      }

      if (chart.current) {
        chart.current.remove();
      }
      if (chartContainerRef.current) {
        // remove all the children because we created the legend elements
        chartContainerRef.current.replaceChildren();
      }

    };
  }, [props.runID, props.reportSummary, currentInterval, showPositionBase, showPositionAverageCost, selectedTimeRange])

  return (
    <div>
      <Group>
        <SegmentedControl
          value={currentInterval}
          data={intervals.map((interval) => {
            return {label: interval, value: interval}
          })}
          onChange={setCurrentInterval}
        />
        <Checkbox label="Position Base" checked={showPositionBase}
                  onChange={(event) => setShowPositionBase(event.currentTarget.checked)}/>
        <Checkbox label="Position Average Cost" checked={showPositionAverageCost}
                  onChange={(event) => setShowPositionAverageCost(event.currentTarget.checked)}/>
      </Group>

      <div ref={chartContainerRef} style={{'flex': 1, 'minHeight': 500, position: 'relative'}}>

      </div>

      <TimeRangeSlider
        selectedInterval={selectedTimeRange}
        timelineInterval={reportTimeRange}
        formatTick={(ms: Date) => format(new Date(ms), 'M d HH')}
        step={1000 * parseInterval(currentInterval)}
        onChange={(tr: any) => {
          console.log("selectedTimeRange", tr)
          setSelectedTimeRange(tr)
        }}
      />

      <OrderListTable orders={orders} onClick={(order) => {
        console.log("selected order", order);
        const visibleRange = chart.current.timeScale().getVisibleRange()
        const seconds = parseInterval(currentInterval)
        const bars = 12
        const orderTime = order.creation_time.getTime() / 1000
        const from = orderTime - bars * seconds
        const to = orderTime + bars * seconds

        console.log("orderTime", orderTime)
        console.log("visibleRange", visibleRange)
        console.log("setVisibleRange", from, to, to - from)
        chart.current.timeScale().setVisibleRange({from, to} as TimeRange);
        // chart.current.timeScale().scrollToPosition(20, true);
      }}/>
    </div>
  );
};

const calculateEMA = (a: KLine[], r: number) => {
  return a.map((k) => {
    return {time: k.time, value: k.close}
  }).reduce((p: any[], n: any, i: number) => {
    if (i) {
      const last = p[p.length - 1]
      const v = 2 * n.value / (r + 1) + last.value * (r - 1) / (r + 1)
      return p.concat({value: v, time: n.time})
    }

    return p
  }, [{
    value: a[0].close,
    time: a[0].time
  }])
}

const createLegend = (i: number, color: string) => {
  const legend = document.createElement('div');
  legend.className = 'ema-legend';
  legend.style.display = 'block';
  legend.style.position = 'absolute';
  legend.style.left = 3 + 'px';
  legend.style.color = color;
  legend.style.zIndex = '99';
  legend.style.top = 3 + (i * 22) + 'px';
  return legend;
}

const createLegendUpdater = (legend: HTMLDivElement, prefix: string) => {
  return (priceValue: any) => {
    let val = '-';
    if (priceValue !== undefined) {
      val = (Math.round(priceValue * 100) / 100).toFixed(2);
    }
    legend.innerHTML = prefix + ' <span>' + val + '</span>';
  }
}

const formatDate = (d: Date): string => {
  return moment(d).format("MMM Do YY hh:mm:ss A Z");
}

const createOHLCLegendUpdater = (legend: HTMLDivElement, prefix: string) => {
  return (param: any, time: any) => {
    if (param) {
      const change = Math.round((param.close - param.open) * 100.0) / 100.0
      const changePercentage = Math.round((param.close - param.open) / param.close * 10000.0) / 100.0;
      const ampl = Math.round((param.high - param.low) / param.low * 10000.0) / 100.0;
      const t = new Date(time * 1000);
      const dateStr = formatDate(t);
      legend.innerHTML = prefix + ` O: ${param.open} H: ${param.high} L: ${param.low} C: ${param.close} CHG: ${change} (${changePercentage}%) AMP: ${ampl}% T: ${dateStr}`;
    } else {
      legend.innerHTML = prefix + ' O: - H: - L: - C: - T: -';
    }
  }
}


export default TradingViewChart;
