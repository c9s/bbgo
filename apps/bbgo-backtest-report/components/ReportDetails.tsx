import React, {useEffect, useState} from 'react';

import TradingViewChart from './TradingViewChart';

import {ReportSummary} from "../types";

import { Title } from '@mantine/core';
import { Button } from '@mantine/core';

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
    return <div>
      <h2>Loading {props.runID}</h2>
    </div>;
  }

  return <div>
    <Title order={2}>RUN {props.runID}</Title>
    <div>
      {
        reportSummary.symbols.map((symbol: string, i : number) => {
          return <TradingViewChart key={i} basePath={props.basePath} runID={props.runID} reportSummary={reportSummary} symbol={symbol} intervals={["1m", "5m", "1h"]}/>
        })
      }
    </div>
  </div>;
};

export default ReportDetails;
