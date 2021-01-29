import React, {useEffect, useState} from 'react';

import {ResponsivePie} from '@nivo/pie';
import {queryAssets} from '../api/bbgo';
import {currencyColor} from '../src/utils';

function reduceAssetsBy(assets, field, minimum) {
    let as = []

    let others = {id: "others", labels: "others", value: 0.0}
    for (let key in assets) {
        if (assets[key]) {
            let a = assets[key]
            let value = a[field]

            if (value < minimum) {
                others.value += value
            } else {
                as.push({
                    id: a.currency,
                    label: a.currency,
                    color: currencyColor(a.currency),
                    value: Math.round(value, 1),
                })
            }
        }
    }

    return as
}

export default function TotalAssetsPie() {
    const [assets, setAssets] = useState({})

    useEffect(() => {
        queryAssets((assets) => {
            setAssets(assets)
        })
    }, [])

    return <ResponsivePie
        data={reduceAssetsBy(assets, "inUSD", 2)}
        margin={{top: 40, right: 80, bottom: 80, left: 80}}
        padding={0.2}
        innerRadius={0.8}
        padAngle={1.0}
        valueFormat=" >-$f"
        colors={{datum: 'data.color'}}
        // colors={{scheme: 'nivo'}}
        cornerRadius={0.1}
        borderWidth={1}
        borderColor={{from: 'color', modifiers: [['darker', 0.2]]}}
        radialLabelsSkipAngle={10}
        radialLabelsTextColor="#333333"
        radialLabelsLinkColor={{from: 'color'}}
        sliceLabelsSkipAngle={30}
        sliceLabelsTextColor="#fff"
        legends={[
            {
                anchor: 'right',
                direction: 'column',
                justify: false,
                translateX: 30,
                translateY: 0,
                itemsSpacing: 5,
                itemWidth: 120,
                itemHeight: 24,
                itemTextColor: '#999',
                itemOpacity: 1,
                symbolSize: 18,
                symbolShape: 'circle',
                effects: [
                    {
                        on: 'hover',
                        style: {
                            itemTextColor: '#000'
                        }
                    }
                ]
            }
        ]}
    />
}
