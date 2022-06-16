import DashboardLayout from '../layouts/DashboardLayout';
import { styled } from '@mui/styles';
import { queryStrategiesMetrics } from '../api/bbgo';
import { useEffect, useState } from 'react';

const StrategyContainer = styled('section')(() => ({
  display: 'flex',
  flexDirection: 'column',
  justifyContent: 'space-around',
  width: '400px',
  height: '400px',
  border: '1px solid rgb(248, 149, 35)',
  borderRadius: '10px',
  padding: '10px',
}));

const Strategy = styled('div')(() => ({
  fontSize: '20px',
}));

const StatusSign = styled('span')(() => ({
  width: '10px',
  height: '10px',
  display: 'block',
  backgroundColor: 'rgb(113, 218, 113)',
  borderRadius: '50%',
  marginRight: '5px',
}));

const RunningTime = styled('div')(() => ({
  display: 'flex',
  alignItems: 'center',
}));

const Description = styled('div')(() => ({
  color: 'rgb(140, 140, 140)',
  '& .duration': {
    marginLeft: '3px',
  },
}));

const Summary = styled('div')(() => ({
  width: '100%',
  display: 'flex',
  justifyContent: 'space-around',
  backgroundColor: 'rgb(255, 245, 232)',
}));

const SummaryBlock = styled('div')(() => ({
  padding: '5px 0 5px 0',
}));

const StatsTitle = styled('div')(() => ({
  margin: '0 0 10px 0',
}));

const StatsValue = styled('div')(() => ({
  marginBottom: '10px',
  color: 'rgb(123, 169, 90)',
}));

const Percentage = styled('div')(() => ({
  color: 'rgb(123, 169, 90)',
}));

const Stats = styled('div')(() => ({
  display: 'grid',
  gridTemplateColumns: '1fr 1fr 1fr',
  columnGap: '10px',
  rowGap: '20px',
}));

export default function Strategies() {
  const [details, setDetails] = useState([]);

  useEffect(() => {
    queryStrategiesMetrics().then((value) => {
      setDetails(value);
    });
  }, []);

  return (
    <DashboardLayout>
      {details.map((element) => {
        return <Detail key={element.id} data={element} />;
      })}
    </DashboardLayout>
  );
}

export function Detail({ data }) {
  const { strategy, stats, startTime } = data;
  const totalProfitsPercentage = (stats.totalProfits / stats.investment) * 100;
  const gridProfitsPercentage = (stats.gridProfits / stats.investment) * 100;
  const gridAprPercentage = (stats.gridProfits / 5) * 365;

  const now = Date.now();
  const durationMilliseconds = now - startTime;
  const seconds = durationMilliseconds / 1000;

  const day = Math.floor(seconds / (60 * 60 * 24));
  const hour = Math.floor((seconds % (60 * 60 * 24)) / 3600);
  const min = Math.floor(((seconds % (60 * 60 * 24)) % 3600) / 60);

  return (
    <StrategyContainer>
      <Strategy>{strategy}</Strategy>
      <div>{data[strategy].symbol}</div>
      <RunningTime>
        <StatusSign />
        <Description>
          Running for
          <span className="duration">{day}</span>D
          <span className="duration">{hour}</span>H
          <span className="duration">{min}</span>M
        </Description>
      </RunningTime>

      <Description>
        0 arbitrages in 24 hours / Total <span>{stats.totalArbs}</span>{' '}
        arbitrages
      </Description>

      <Summary>
        <SummaryBlock>
          <StatsTitle>Investment USDT</StatsTitle>
          <div>{stats.investment}</div>
        </SummaryBlock>

        <SummaryBlock>
          <StatsTitle>Total Profit USDT</StatsTitle>
          <StatsValue>{stats.totalProfits}</StatsValue>
          <Percentage>{totalProfitsPercentage}%</Percentage>
        </SummaryBlock>
      </Summary>

      <Stats>
        <div>
          <StatsTitle>Grid Profits</StatsTitle>
          <StatsValue>{stats.gridProfits}</StatsValue>
          <Percentage>{gridProfitsPercentage}%</Percentage>
        </div>

        <div>
          <StatsTitle>Floating PNL</StatsTitle>
          <StatsValue>{stats.floatingPNL}</StatsValue>
        </div>

        <div>
          <StatsTitle>Grid APR</StatsTitle>
          <Percentage>{gridAprPercentage}%</Percentage>
        </div>

        <div>
          <StatsTitle>Current Price</StatsTitle>
          <div>{stats.currentPrice}</div>
        </div>

        <div>
          <StatsTitle>Price Range</StatsTitle>
          <div>
            {stats.lowestPrice}~{stats.highestPrice}
          </div>
        </div>
      </Stats>
    </StrategyContainer>
  );
}
