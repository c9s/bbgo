import { styled } from '@mui/styles';
import { Description } from './Detail';

const RunningTimeSection = styled('div')(() => ({
  display: 'flex',
  alignItems: 'center',
}));

const StatusSign = styled('span')(() => ({
  width: '10px',
  height: '10px',
  display: 'block',
  backgroundColor: 'rgb(113, 218, 113)',
  borderRadius: '50%',
  marginRight: '5px',
}));

export default function RunningTime({ seconds }: { seconds: number }) {
  const day = Math.floor(seconds / (60 * 60 * 24));
  const hour = Math.floor((seconds % (60 * 60 * 24)) / 3600);
  const min = Math.floor(((seconds % (60 * 60 * 24)) % 3600) / 60);

  return (
    <RunningTimeSection>
      <StatusSign />
      <Description>
        Running for
        <span className="duration">{day}</span>D
        <span className="duration">{hour}</span>H
        <span className="duration">{min}</span>M
      </Description>
    </RunningTimeSection>
  );
}
