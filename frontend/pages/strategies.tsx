import DashboardLayout from '../layouts/DashboardLayout';
import { styled } from '@mui/styles';

const StrategyContainer = styled('section')(() => ({
  display: 'flex',
  flexDirection: 'column',
  justifyContent: 'space-around',
  width: '400px',
  height: '400px',
  border: '1px solid rgb(248, 149, 35)',
  borderRadius: '10px',
  padding: '10px'
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
  marginRight:'5px',
}));

const RunningTime = styled('div')(() => ({
  display: 'flex',
  alignItems: 'center',
}))

const Description = styled('div')(() => ({
  color: 'rgb(140, 140, 140)',
}))

const Summary = styled('div')(() => ({
  width: '100%',
  display: 'flex',
  justifyContent: 'space-around',
  backgroundColor: 'rgb(255, 245, 232)',
}))

const SummaryBlock = styled('div')(() => ({
  padding: '5px 0 5px 0'

}));

const StatsTitle = styled('div')(() => ({
  margin:'0 0 10px 0',

}));

const StatsValue = styled('div')(() => ({
  marginBottom: '10px',
  color: 'rgb(123, 169, 90)',
}));

const Precentage = styled('div')(() => ({
  color: 'rgb(123, 169, 90)',
}));

const Stats = styled('div')(() => ({
  display: 'grid',
  gridTemplateColumns: '1fr 1fr 1fr',
  columnGap: '10px',
  rowGap: '20px'
}));


export default function Strategies(){

  return (
    <DashboardLayout>
      <StrategyContainer>
        <Strategy>Grid Trading</Strategy>
        <div>BTC/USDT</div>
        <RunningTime>
          <StatusSign/>
          <Description>Running for 5D 0H 0M</Description>
        </RunningTime> 

        <Description>0 arbitrages in 24 hours / Total 3 arbitrages</Description>
        
        <Summary>
          <SummaryBlock>
           <StatsTitle>Investment USDT</StatsTitle>
            <div>100</div>
          </SummaryBlock>
          
          <SummaryBlock>
            <StatsTitle>Total Profit USDT</StatsTitle>
            <StatsValue>+5.600</StatsValue>
            <Precentage>+5.6%</Precentage>
          </SummaryBlock>
        </Summary>

        <Stats>
            <div>
              <StatsTitle>Grid Profits</StatsTitle>
              <StatsValue>+2.500</StatsValue>
              <Precentage>+2.5%</Precentage>
            </div>

            <div>
              <StatsTitle>Floating PNL</StatsTitle>
              <StatsValue>+3.100</StatsValue>
            </div>

            <div>
              <StatsTitle>Grid APR</StatsTitle>
              <Precentage>+182.5%</Precentage>
            </div>
 
            <div>
              <StatsTitle>Current Price</StatsTitle>
              <div>29,000</div>
            </div>

            <div>
              <StatsTitle>Price Range</StatsTitle>
              <div>25,000~35,000</div>
            </div>
          </Stats>
      </StrategyContainer>
      
    </DashboardLayout>

  );
}