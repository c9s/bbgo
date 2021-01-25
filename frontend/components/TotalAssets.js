import React, {useEffect, useState} from 'react';

import {ResponsivePie} from '@nivo/pie'
import axios from 'axios';

function currencyColor(currency) {
    switch (currency) {
        case "BTC":
            return "#f69c3d"
        case "ETH":
            return "#497493"
        case "MCO":
            return "#032144"
        case "OMG":
            return "#2159ec"
        case "LTC":
            return "#949494"
        case "USDT":
            return "#2ea07b"
        case "SAND":
            return "#2E9AD0"
        case "XRP":
            return "#00AAE4"
        case "BCH":
            return "#8DC351"
        case "MAX":
            return "#2D4692"
        case "TWD":
            return "#4A7DED"

    }
}

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

export default function TotalAssets() {
    const [assets, setAssets] = useState({})

    useEffect(function loadAssets() {
        axios.get('http://localhost:8080/api/assets', {})
            .then(response => {
                console.log(response.data.assets);
                setAssets(response.data.assets)
            });

    }, [])

    return <ResponsivePie
        data={reduceAssetsBy(assets, "inUSD", 2)}
        margin={{top: 40, right: 80, bottom: 80, left: 80}}
        padding={0.2}
        innerRadius={0.8}
        padAngle={1.0}
        colors={{datum: 'data.color'}}
        // colors={{scheme: 'nivo'}}
        cornerRadius={0.0}
        borderWidth={1}
        borderColor={{from: 'color', modifiers: [['darker', 0.2]]}}
        radialLabelsSkipAngle={10}
        radialLabelsTextColor="#333333"
        radialLabelsLinkColor={{from: 'color'}}
        sliceLabelsSkipAngle={10}
        sliceLabelsTextColor="#fff"
        legends={[
            {
                anchor: 'bottom',
                direction: 'row',
                justify: false,
                translateX: 0,
                translateY: 56,
                itemsSpacing: 0,
                itemWidth: 100,
                itemHeight: 18,
                itemTextColor: '#999',
                itemDirection: 'left-to-right',
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
