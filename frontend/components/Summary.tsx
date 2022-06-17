import { styled } from '@mui/styles';
import { GridStats } from '../api/bbgo';

const SummarySection = styled('div')(() => ({
  width: '100%',
  display: 'flex',
  justifyContent: 'space-around',
  backgroundColor: 'rgb(255, 245, 232)',
  margin: '10px 0',
}));

const SummaryBlock = styled('div')(() => ({
  padding: '5px 0 5px 0',
}));

export const StatsTitle = styled('div')(() => ({
  margin: '0 0 10px 0',
}));

export const StatsValue = styled('div')(() => ({
  marginBottom: '10px',
  color: 'rgb(123, 169, 90)',
}));

export const Percentage = styled('div')(() => ({
  color: 'rgb(123, 169, 90)',
}));

export default function Summary({
  stats,
  totalProfitsPercentage,
}: {
  stats: GridStats;
  totalProfitsPercentage: number;
}) {
  return (
    <SummarySection>
      <SummaryBlock>
        <StatsTitle>Investment USDT</StatsTitle>
        <div>{stats.investment}</div>
      </SummaryBlock>

      <SummaryBlock>
        <StatsTitle>Total Profit USDT</StatsTitle>
        <StatsValue>{stats.totalProfits}</StatsValue>
        <Percentage>{totalProfitsPercentage}%</Percentage>
      </SummaryBlock>
    </SummarySection>
  );
}
