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
        <Group position="apart">
          <div>
            <Text
              color="dimmed"
              transform="uppercase"
              weight={700}
              size="xs"
              className={classes.label}
            >
              {stat.title}
            </Text>
            <Text weight={700} size="xl">
              {stat.value}
            </Text>
          </div>

          {stat.dir ?
            <ThemeIcon
              color="gray"
              variant="light"
              sx={(theme) => ({color: stat.dir == "up" ? theme.colors.teal[6] : theme.colors.red[6]})}
              size={38}
              radius="md"
            >
              <DirIcon size={28}/>
            </ThemeIcon>
            : null}

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
    <div className={classes.root}>
      <SimpleGrid cols={4} breakpoints={[{maxWidth: 'sm', cols: 1}]}>
        {stats}
      </SimpleGrid>
    </div>
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

  return <Table>
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

  const totalProfit = Math.round(reportSummary.symbolReports.map((report) => report.pnl.profit).reduce((prev, cur) => prev + cur) * 100) / 100
  const totalUnrealizedProfit = Math.round(reportSummary.symbolReports.map((report) => report.pnl.unrealizedProfit).reduce((prev, cur) => prev + cur) * 100) / 100
  const totalTrades = reportSummary.symbolReports.map((report) => report.pnl.numTrades).reduce((prev, cur) => prev + cur) || 0

  const totalBuyVolume = reportSummary.symbolReports.map((report) => report.pnl.buyVolume).reduce((prev, cur) => prev + cur) || 0
  const totalSellVolume = reportSummary.symbolReports.map((report) => report.pnl.sellVolume).reduce((prev, cur) => prev + cur) || 0

  const volumeUnit = reportSummary.symbolReports.length == 1 ? reportSummary.symbolReports[0].market.baseCurrency : '';

  return <div>
    <Container my="md" mx="xs">
      <Title order={2}>RUN {props.runID}</Title>
      <div>
        {reportSummary.sessions.map((session) => <Badge key={session}>Exchange {session}</Badge>)}
        {reportSummary.symbols.map((symbol) => <Badge key={symbol}>{symbol}</Badge>)}

        <Badge>{reportSummary.startTime.toString()} ~ {reportSummary.endTime.toString()}</Badge>
        <Badge>{
          moment.duration((new Date(reportSummary.endTime)).getTime() - (new Date(reportSummary.startTime)).getTime()).humanize()
        }</Badge>
      </div>
      <StatsGridIcons data={[
        {title: "Profit", value: "$" + totalProfit.toString(), dir: totalProfit > 0 ? "up" : "down"},
        {
          title: "Unrealized Profit",
          value: "$" + totalUnrealizedProfit.toString(),
          dir: totalUnrealizedProfit > 0 ? "up" : "down"
        },
        {title: "Trades", value: totalTrades.toString()},
        {title: "Buy Volume", value: totalBuyVolume.toString() + ` ${volumeUnit}`},
        {title: "Sell Volume", value: totalSellVolume.toString() + ` ${volumeUnit}`},
      ]}/>

      <Grid p={"xs"} mb={"lg"}>
        <Grid.Col xs={6}>
          <Title order={5}>Initial Total Balances</Title>
          <BalanceDetails balances={reportSummary.initialTotalBalances}/>
        </Grid.Col>
        <Grid.Col xs={6}>
          <Title order={5}>Final Total Balances</Title>
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

    </Container>
  </div>;
};

export default ReportDetails;
