import DashboardLayout from '../layouts/DashboardLayout';
import { useEffect, useState } from 'react';
import { queryStrategiesMetrics } from '../api/bbgo';
import type { GridStrategy } from '../api/bbgo';

import Detail from '../components/Detail';


export default function Strategies() {
  const [details, setDetails] = useState<GridStrategy[]>([]);

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
