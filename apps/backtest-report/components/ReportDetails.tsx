import React, {useEffect, useState} from 'react';

import moment from 'moment';

import TradingViewChart from './TradingViewChart';

import {BalanceMap, ReportSummary} from "../types";

import {
  Badge,
  Container,
  createStyles,
  Grid,
  Group,
  Paper,
  SimpleGrid,
  Skeleton,
  Table,
  Text,
  ThemeIcon,
  Title
} from '@mantine/core';

import {ArrowDownRight, ArrowUpRight,} from 'tabler-icons-react';

const useStyles = createStyles((theme) => ({
  root: {
    paddingTop: theme.spacing.xl * 1.5,
    paddingBottom: theme.spacing.xl * 1.5,
  },

  label: {
    fontFamily: `Greycliff CF, ${theme.fontFamily}`,
  },
}));

interface StatsGridIconsProps {
  data: {
    title: string;
    value: string;
    diff?: number
    dir?: string;
    desc?: string;
  }[];
}

function StatsGridIcons({data}: StatsGridIconsProps) {
  const {classes} = useStyles();
  const stats = data.map((stat) => {
    const DiffIcon = stat.diff && stat.diff > 0 ? ArrowUpRight : ArrowDownRight;
    const DirIcon = stat.dir && stat.dir == "up" ? ArrowUpRight : ArrowDownRight;

  return (
      <Paper withBorder p="xs" radius="md" key={stat.title}>
        <Group position="left">
          <div>
            <Text
              color="dimmed"
              weight={700}
              size="xs"
              className={classes.label}
            >
              {stat.title}
              {stat.dir ?
                <ThemeIcon
                  color="gray"
                  variant="light"
                  sx={(theme) => ({color: stat.dir == "up" ? theme.colors.teal[6] : theme.colors.red[6]})}
                  size={16}
                  radius="xs"
                >
                  <DirIcon size={16}/>
                </ThemeIcon>
                : null}
            </Text>
            <Text weight={700} size="xs">
              {stat.value}
            </Text>
          </div>


          {stat.diff ?
            <ThemeIcon
              color="gray"
              variant="light"
              sx={(theme) => ({color: stat.diff && stat.diff > 0 ? theme.colors.teal[6] : theme.colors.red[6]})}
              size={38}
              radius="md"
            >
              <DiffIcon size={28}/>
            </ThemeIcon>
            : null}
        </Group>

        {stat.diff ?
          <Text color="dimmed" size="sm" mt="md">
            <Text component="span" color={stat.diff && stat.diff > 0 ? 'teal' : 'red'} weight={700}>
              {stat.diff}%
            </Text>{' '}
            {stat.diff && stat.diff > 0 ? 'increase' : 'decrease'} compared to last month
          </Text> : null}

        {stat.desc ? (
          <Text color="dimmed" size="sm" mt="md">
            {stat.desc}
          </Text>
        ) : null}

      </Paper>
    );
  });

  return (
      <SimpleGrid cols={5} breakpoints={[{maxWidth: 'sm', cols: 1, spacing: 'xl'}]} py="xl">
        {stats}
      </SimpleGrid>
  );
}


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

const skeleton = <Skeleton height={140} radius="md" animate={false}/>;


interface BalanceDetailsProps {
  balances: BalanceMap;
}

const BalanceDetails = (props: BalanceDetailsProps) => {
  const rows = Object.entries(props.balances).map(([k, v]) => {
    return <tr key={k}>
      <td>{k}</td>
      <td>{v.available}</td>
    </tr>;
  });

  return <Table verticalSpacing="xs" fontSize="xs">
    <thead>
    <tr>
      <th>Currency</th>
      <th>Balance</th>
    </tr>
    </thead>
    <tbody>{rows}</tbody>
  </Table>;
};

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

  const strategyName = props.runID.split("_")[1]
  const runID = props.runID.split("_").pop()
  const totalProfit = Math.round(reportSummary.symbolReports.map((report) => report.pnl.profit).reduce((prev, cur) => prev + cur) * 100) / 100
  const totalUnrealizedProfit = Math.round(reportSummary.symbolReports.map((report) => report.pnl.unrealizedProfit).reduce((prev, cur) => prev + cur) * 100) / 100
  const totalTrades = reportSummary.symbolReports.map((report) => report.pnl.numTrades).reduce((prev, cur) => prev + cur) || 0

  const totalBuyVolume = reportSummary.symbolReports.map((report) => report.pnl.buyVolume).reduce((prev, cur) => prev + cur) || 0
  const totalSellVolume = reportSummary.symbolReports.map((report) => report.pnl.sellVolume).reduce((prev, cur) => prev + cur) || 0

  const volumeUnit = reportSummary.symbolReports.length == 1 ? reportSummary.symbolReports[0].market.baseCurrency : '';



  // size xl and padding xs
  return <Container size={"xl"} px="xs">
      <div>
        <Badge key={strategyName} color="teal">Strategy: {strategyName}</Badge>
        {reportSummary.sessions.map((session) => <Badge key={session} color="teal">Exchange: {session}</Badge>)}
        {reportSummary.symbols.map((symbol) => <Badge key={symbol} color="teal">Symbol: {symbol}</Badge>)}

        <Badge color="teal">{reportSummary.startTime.toString()} — {reportSummary.endTime.toString()} ~ {
          moment.duration((new Date(reportSummary.endTime)).getTime() - (new Date(reportSummary.startTime)).getTime()).humanize()
        }</Badge>
        <Badge key={runID} color="gray" size="xs">Run ID: {runID}</Badge>
      </div>
      <StatsGridIcons data={[
        {title: "Profit", value: "$" + totalProfit.toString(), dir: totalProfit >= 0 ? "up" : "down"},
        {
          title: "Unr. Profit",
          value: totalUnrealizedProfit.toString() + "$",
          dir: totalUnrealizedProfit > 0 ? "up" : "down"
        },
        {title: "Trades", value: totalTrades.toString()},
        {title: "Buy Vol", value: totalBuyVolume.toString() + ` ${volumeUnit}`},
        {title: "Sell Vol", value: totalSellVolume.toString() + ` ${volumeUnit}`},
      ]}/>

      <Grid py="xl">
        <Grid.Col xs={6}>
          <Title order={6}>Initial Total Balances</Title>
          <BalanceDetails balances={reportSummary.initialTotalBalances}/>
        </Grid.Col>
        <Grid.Col xs={6}>
          <Title order={6}>Final Total Balances</Title>
          <BalanceDetails balances={reportSummary.finalTotalBalances}/>
        </Grid.Col>
      </Grid>

      {
        /*
        <Grid>
          <Grid.Col span={6}>
            <Skeleton height={300} radius="md" animate={false}/>
          </Grid.Col>
          <Grid.Col xs={4}>{skeleton}</Grid.Col>
        </Grid>
        */
      }
      <div>
        {
          reportSummary.symbols.map((symbol: string, i: number) => {
            return <TradingViewChart key={i} basePath={props.basePath} runID={props.runID} reportSummary={reportSummary}
                                     symbol={symbol}/>
          })
        }
      </div>

    </Container>;
};


export default ReportDetails;
