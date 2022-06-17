import { styled } from '@mui/styles';
import { StatsTitle, StatsValue, Percentage } from './Summary';
import { GridStats } from '../api/bbgo';

const StatsSection = styled('div')(() => ({
  display: 'grid',
  gridTemplateColumns: '1fr 1fr 1fr',
  gap: '10px',
}));

export default function Stats({
  stats,
  gridProfitsPercentage,
  gridAprPercentage,
}: {
  stats: GridStats;
  gridProfitsPercentage: number;
  gridAprPercentage: number;
}) {
  return (
    <StatsSection>
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
    </StatsSection>
  );
}
