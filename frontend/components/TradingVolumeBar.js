import {ResponsiveBar} from '@nivo/bar';
import {queryTradingVolume} from '../api/bbgo';
import {useEffect, useState} from "react";


function groupData(rows) {
    let dateIndex = {}
    let startTime = null
    let endTime = null
    rows.forEach((v) => {
        const time = new Date(v.time)
        if (!startTime) {
            startTime = time
        }

        endTime = time

        const dateStr = time.getFullYear() + "-" + (time.getMonth() + 1) + "-" + time.getDate()
        const key = v.exchange
        const k = key ? key : "total"
        const quoteVolume = Math.round(v.quoteVolume * 100) / 100

        if (dateIndex[dateStr]) {
            dateIndex[dateStr][k] = quoteVolume
        } else {
            dateIndex[dateStr] = {
                date: dateStr,
                year: time.getFullYear(),
                month: time.getMonth() + 1,
                day: time.getDate(),
                [k]: quoteVolume,
            }
        }
    })

    let data = []
    while (startTime < endTime) {
        const dateStr = startTime.getFullYear() + "-" + (startTime.getMonth() + 1) + "-" + startTime.getDate()
        const groupData = dateIndex[dateStr]
        if (groupData) {
            data.push(groupData)
        } else {
            data.push({
                date: dateStr,
                year: startTime.getFullYear(),
                month: startTime.getMonth() + 1,
                day: startTime.getDate(),
                total: 0,
            })
        }

        startTime.setDate(startTime.getDate() + 1)
    }

    return data
}

export default function TradingVolumeBar() {
    const [tradingVolumes, setTradingVolumes] = useState([])

    useEffect(() => {
        queryTradingVolume({}, (tradingVolumes) => {
            setTradingVolumes(tradingVolumes)
        })
    }, [])

    const data = groupData(tradingVolumes)

    return <ResponsiveBar keys={["total"]}
                          data={data}
                          indexBy={"date"}
                          margin={{ top: 50, right: 160, bottom: 50, left: 60 }}
                          padding={0.3}
                          valueScale={{ type: 'linear' }}
                          indexScale={{ type: 'band', round: true }}
                          enableGridY={true}
                          colors={{ scheme: 'paired' }}
                          legends={[
                              {
                                  dataFrom: 'keys',
                                  anchor: 'right',
                                  direction: 'column',
                                  justify: false,
                                  translateX: 120,
                                  translateY: 0,
                                  itemsSpacing: 2,
                                  itemWidth: 100,
                                  itemHeight: 20,
                                  itemDirection: 'left-to-right',
                                  itemOpacity: 0.85,
                                  symbolSize: 20,
                                  effects: [
                                      {
                                          on: 'hover',
                                          style: {
                                              itemOpacity: 1
                                          }
                                      }
                                  ]
                              }
                          ]}
                          animate={true}
                          motionStiffness={90}
                          motionDamping={15}
    />;
}
