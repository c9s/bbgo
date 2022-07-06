import type {NextPage} from 'next'
import styles from '../styles/Home.module.css'
import {useRouter} from "next/router";
import {useEffect, useState} from "react";
import dynamic from 'next/dynamic';

import * as d3 from "d3";

const DynamicPlot = dynamic(import('../components/Plot'), {
  ssr: false
})

const Metrics: NextPage = () => {
  const {query} = useRouter();
  const basePath = query.basePath ? query.basePath as string : '/output';
  const [zData, setZData] = useState();

  useEffect(() => {
    d3.csv('https://raw.githubusercontent.com/plotly/datasets/master/api_docs/mt_bruno_elevation.csv').then((rows) => {
      function unpack(rows, key) {
        return rows.map(function (row) {
          return row[key];
        });
      }

      let z_data = []
      for (let i = 0; i < 24; i++) {
        z_data.push(unpack(rows, i));
      }
      setZData(z_data);
    });
  }, [])

  return (
    <div>
      <main className={styles.main}>
        <DynamicPlot
          data={[{
            z: zData,
            type: 'surface'
          }]}
          layout={{
            title: 'Metrics',
            autosize: true,
            width: 800,
            height: 500,
            margin: {
              l: 65,
              r: 50,
              b: 65,
              t: 90,
            }
          }}
        />
      </main>
    </div>
  )
}

export default Metrics
