import React from 'react';

import TradingViewChart from './TradingViewChart';

import {Container} from '@nextui-org/react';

const Report = (props) => {
  /*
   <Button>Click me</Button>
   */
  return <Container>
    <h2>Back-test Report</h2>
    <div>
      <TradingViewChart intervals={["1m", "5m"]}/>
    </div>
  </Container>;
};

export default Report;
