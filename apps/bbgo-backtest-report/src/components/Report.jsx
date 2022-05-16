import React from 'react';

import TradingViewChart from './TradingViewChart';

import { Button } from '@nextui-org/react';

const Report = (props) => {
  /*
   <Button>Click me</Button>
   */
  return <div>
    <h2>Back-test Report</h2>
    <div>
      <TradingViewChart/>
    </div>
  </div>;
};

export default Report;
