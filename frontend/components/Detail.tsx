import { styled } from '@mui/styles';
import type { GridStrategy } from '../api/bbgo';

import RunningTime from './RunningTime';
import Summary from './Summary';
import Stats from './Stats';

const StrategyContainer = styled('section')(() => ({
  display: 'flex',
  flexDirection: 'column',
  justifyContent: 'space-around',
  width: '350px',
  border: '1px solid rgb(248, 149, 35)',
  borderRadius: '10px',
  padding: '10px',
}));

const Strategy = styled('div')(() => ({
  fontSize: '20px',
}));

export const Description = styled('div')(() => ({
  color: 'rgb(140, 140, 140)',
  '& .duration': {
    marginLeft: '3px',
  },
}));

export default function Detail({ data }: { data: GridStrategy }) {
  const { strategy, stats, startTime } = data;
  const totalProfitsPercentage = (stats.totalProfits / stats.investment) * 100;
  const gridProfitsPercentage = (stats.gridProfits / stats.investment) * 100;
  const gridAprPercentage = (stats.gridProfits / 5) * 365;

  const now = Date.now();
  const durationMilliseconds = now - startTime;
  const seconds = durationMilliseconds / 1000;

  return (
    <StrategyContainer>
      <Strategy>{strategy}</Strategy>
      <div>{data[strategy].symbol}</div>
      <RunningTime seconds={seconds} />
      <Description>
        0 arbitrages in 24 hours / Total <span>{stats.totalArbs}</span>{' '}
        arbitrages
      </Description>
      <Summary stats={stats} totalProfitsPercentage={totalProfitsPercentage} />
      <Stats
        stats={stats}
        gridProfitsPercentage={gridProfitsPercentage}
        gridAprPercentage={gridAprPercentage}
      />
    </StrategyContainer>
  );
}
