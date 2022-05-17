import React, {useEffect, useState} from 'react';

import TradingViewChart from './TradingViewChart';

import {Container} from '@nextui-org/react';
import {ReportSummary} from "../types";

interface ReportDetailsProps {
  basePath: string;
  runID: string;
}

const fetchReportSummary = (basePath: string, runID: string) => {
  return fetch(
    `${basePath}/${runID}/summary.json`,
  )
    .then((res) => res.json())
    .catch((e) => {
      console.error("failed to fetch index", e)
    });
}

const ReportDetails = (props: ReportDetailsProps) => {
  const [reportSummary, setReportSummary] = useState<ReportSummary>()
  useEffect(() => {
    fetchReportSummary(props.basePath, props.runID).then((summary: ReportSummary) => {
      console.log("summary", props.runID, summary);
      setReportSummary(summary)
    })
  }, [props.runID])

  if (!reportSummary) {
    return <Container>
      <h2>Loading {props.runID}</h2>
    </Container>;
  }

  return <Container>
    <h2>Back-test Run {props.runID}</h2>
    <div>
      {
        reportSummary.symbols.map((symbol: string) => {
          return <TradingViewChart basePath={props.basePath} runID={props.runID} reportSummary={reportSummary} symbol={symbol} intervals={["1m", "5m", "1h"]}/>
        })
      }

    </div>
  </Container>;
};

export default ReportDetails;
