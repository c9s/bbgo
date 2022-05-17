import { format } from "d3-format";
import { tsvParse } from "d3-dsv";
import { timeFormat } from "d3-time-format";
import { timeParse } from "d3-time-format";
import * as React from "react";
import {
  elderRay,
  ema,
  discontinuousTimeScaleProviderBuilder,
  Chart,
  ChartCanvas,
  CurrentCoordinate,
  BarSeries,
  CandlestickSeries,
  ElderRaySeries,
  LineSeries,
  MovingAverageTooltip,
  OHLCTooltip,
  SingleValueTooltip,
  lastVisibleItemBasedZoomAnchor,
  XAxis,
  YAxis,
  CrossHairCursor,
  EdgeIndicator,
  MouseCoordinateX,
  MouseCoordinateY,
  ZoomButtons,
  withDeviceRatio,
  withSize,
} from "react-financial-charts";

interface IOHLCData {
  readonly close: number;
  readonly date: Date;
  readonly high: number;
  readonly low: number;
  readonly open: number;
  readonly volume: number;
}


const parseDate = timeParse("%Y-%m-%d");

const parseData = () => {
  return (d: any) => {
    const date = parseDate(d.date);
    if (date === null) {
      d.date = new Date(Number(d.date));
    } else {
      d.date = new Date(date);
    }

    for (const key in d) {
      if (key !== "date" && Object.prototype.hasOwnProperty.call(d, key)) {
        d[key] = +d[key];
      }
    }

    return d as IOHLCData;
  };
};

interface WithOHLCDataProps {
  readonly data: IOHLCData[];
}

interface WithOHLCState {
  data?: IOHLCData[];
  message: string;
}

export function withOHLCData(interval : string = "5m") {
  return <TProps extends WithOHLCDataProps>(OriginalComponent: React.ComponentClass<TProps>) => {
    return class WithOHLCData extends React.Component<Omit<TProps, "data">, WithOHLCState> {
      public constructor(props: Omit<TProps, "data">) {
        super(props);

        this.state = {
          message: `Loading price data...`,
        };
      }

      public componentDidMount() {
        fetch(
          `/data/klines/ETHUSDT-5m.csv`,
        )
          .then((response) => response.text())
          .then((data) => tsvParse(data, parseData()))
          .then((data) => {
            this.setState({
              data,
            });
          })
          .catch(() => {
            this.setState({
              message: `Failed to fetch data.`,
            });
          });
      }

      public render() {
        const { data, message } = this.state;
        if (data === undefined) {
          return <div className="center">{message}</div>;
        }

        return <OriginalComponent {...(this.props as TProps)} data={data} />;
      }
    };
  };
}


interface StockChartProps {
  readonly data: IOHLCData[];
  readonly height: number;
  readonly dateTimeFormat?: string;
  readonly width: number;
  readonly ratio: number;
}

class StockChart extends React.Component<StockChartProps> {
  private readonly margin = { left: 0, right: 48, top: 0, bottom: 24 };
  private readonly pricesDisplayFormat = format(".2f");
  private readonly xScaleProvider = discontinuousTimeScaleProviderBuilder().inputDateAccessor(
    (d: IOHLCData) => d.date,
  );

  public render() {
    const { data: initialData, dateTimeFormat = "%d %b", height, ratio, width } = this.props;

    const ema12 = ema()
      .id(1)
      .options({ windowSize: 12 })
      .merge((d: any, c: any) => {
        d.ema12 = c;
      })
      .accessor((d: any) => d.ema12);

    const ema26 = ema()
      .id(2)
      .options({ windowSize: 26 })
      .merge((d: any, c: any) => {
        d.ema26 = c;
      })
      .accessor((d: any) => d.ema26);

    const elder = elderRay();

    const calculatedData = elder(ema26(ema12(initialData)));

    const { margin, xScaleProvider } = this;

    const { data, xScale, xAccessor, displayXAccessor } = xScaleProvider(calculatedData);

    const max = xAccessor(data[data.length - 1]);
    const min = xAccessor(data[Math.max(0, data.length - 100)]);
    const xExtents = [min, max + 5];

    const gridHeight = height - margin.top - margin.bottom;

    const elderRayHeight = 100;
    const elderRayOrigin = (_: number, h: number) => [0, h - elderRayHeight];
    const barChartHeight = gridHeight / 4;
    const barChartOrigin = (_: number, h: number) => [0, h - barChartHeight - elderRayHeight];
    const chartHeight = gridHeight - elderRayHeight;

    const timeDisplayFormat = timeFormat(dateTimeFormat);

    return (
      <ChartCanvas
        height={height}
        ratio={ratio}
        width={width}
        margin={margin}
        data={data}
        displayXAccessor={displayXAccessor}
        seriesName="Data"
        xScale={xScale}
        xAccessor={xAccessor}
        xExtents={xExtents}
        zoomAnchor={lastVisibleItemBasedZoomAnchor}
      >
        <Chart id={2} height={barChartHeight} origin={barChartOrigin} yExtents={this.barChartExtents}>
          <BarSeries fillStyle={this.volumeColor} yAccessor={this.volumeSeries} />
        </Chart>
        <Chart id={3} height={chartHeight} yExtents={this.candleChartExtents}>
          <XAxis showGridLines showTicks={false} showTickLabel={false} />
          <YAxis showGridLines tickFormat={this.pricesDisplayFormat} />
          <CandlestickSeries />
          <LineSeries yAccessor={ema26.accessor()} strokeStyle={ema26.stroke()} />
          <CurrentCoordinate yAccessor={ema26.accessor()} fillStyle={ema26.stroke()} />
          <LineSeries yAccessor={ema12.accessor()} strokeStyle={ema12.stroke()} />
          <CurrentCoordinate yAccessor={ema12.accessor()} fillStyle={ema12.stroke()} />
          <MouseCoordinateY rectWidth={margin.right} displayFormat={this.pricesDisplayFormat} />
          <EdgeIndicator
            itemType="last"
            rectWidth={margin.right}
            fill={this.openCloseColor}
            lineStroke={this.openCloseColor}
            displayFormat={this.pricesDisplayFormat}
            yAccessor={this.yEdgeIndicator}
          />
          <MovingAverageTooltip
            origin={[8, 24]}
            options={[
              {
                yAccessor: ema26.accessor(),
                type: "EMA",
                stroke: ema26.stroke(),
                windowSize: ema26.options().windowSize,
              },
              {
                yAccessor: ema12.accessor(),
                type: "EMA",
                stroke: ema12.stroke(),
                windowSize: ema12.options().windowSize,
              },
            ]}
          />

          <ZoomButtons />
          <OHLCTooltip origin={[8, 16]} />
        </Chart>
        <Chart
          id={4}
          height={elderRayHeight}
          yExtents={[0, elder.accessor()]}
          origin={elderRayOrigin}
          padding={{ top: 8, bottom: 8 }}
        >
          <XAxis showGridLines gridLinesStrokeStyle="#e0e3eb" />
          <YAxis ticks={4} tickFormat={this.pricesDisplayFormat} />

          <MouseCoordinateX displayFormat={timeDisplayFormat} />
          <MouseCoordinateY rectWidth={margin.right} displayFormat={this.pricesDisplayFormat} />

          <ElderRaySeries yAccessor={elder.accessor()} />

          <SingleValueTooltip
            yAccessor={elder.accessor()}
            yLabel="Elder Ray"
            yDisplayFormat={(d: any) =>
              `${this.pricesDisplayFormat(d.bullPower)}, ${this.pricesDisplayFormat(d.bearPower)}`
            }
            origin={[8, 16]}
          />
        </Chart>
        <CrossHairCursor />
      </ChartCanvas>
    );
  }

  private readonly barChartExtents = (data: IOHLCData) => {
    return data.volume;
  };

  private readonly candleChartExtents = (data: IOHLCData) => {
    return [data.high, data.low];
  };

  private readonly yEdgeIndicator = (data: IOHLCData) => {
    return data.close;
  };

  private readonly volumeColor = (data: IOHLCData) => {
    return data.close > data.open ? "rgba(38, 166, 154, 0.3)" : "rgba(239, 83, 80, 0.3)";
  };

  private readonly volumeSeries = (data: IOHLCData) => {
    return data.volume;
  };

  private readonly openCloseColor = (data: IOHLCData) => {
    return data.close > data.open ? "#26a69a" : "#ef5350";
  };
}

export default withOHLCData()(withSize({ style: { minHeight: 400 } })(withDeviceRatio()(StockChart)));

export const MinutesStockChart = withOHLCData("MINUTES")(
  withSize({ style: { minHeight: 400 } })(withDeviceRatio()(StockChart)),
);

export const SecondsStockChart = withOHLCData("SECONDS")(
  withSize({ style: { minHeight: 400 } })(withDeviceRatio()(StockChart)),
);
