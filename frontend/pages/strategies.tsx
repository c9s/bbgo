import { styled } from '@mui/styles';
import DashboardLayout from '../layouts/DashboardLayout';
import { useEffect, useState } from 'react';
import { queryStrategiesMetrics } from '../api/bbgo';
import type { GridStrategy } from '../api/bbgo';

import Detail from '../components/Detail';

const StrategiesContainer = styled('div')(() => ({
  width: '100%',
  height: '100%',
  padding: '40px 20px',
  display: 'grid',
  gridTemplateColumns: 'repeat(3, 350px);',
  justifyContent: 'center',
  gap: '30px',
  '@media(max-width: 1400px)': {
    gridTemplateColumns: 'repeat(2, 350px)',
  },
  '@media(max-width: 1000px)': {
    gridTemplateColumns: '350px',
  },
}));

export default function Strategies() {
  const [details, setDetails] = useState<GridStrategy[]>([]);

  useEffect(() => {
    queryStrategiesMetrics().then((value) => {
      setDetails(value);
    });
  }, []);

  return (
    <DashboardLayout>
      <StrategiesContainer>
        {details.map((element) => {
          return <Detail key={element.id} data={element} />;
        })}
      </StrategiesContainer>
    </DashboardLayout>
  );
}
