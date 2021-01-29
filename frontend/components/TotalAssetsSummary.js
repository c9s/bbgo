import {useEffect, useState} from "react";
import {queryAssets} from "../api/bbgo";
import Card from '@material-ui/core/Card';
import CardContent from '@material-ui/core/CardContent';
import Typography from '@material-ui/core/Typography';
import {makeStyles} from '@material-ui/core/styles';

function aggregateAssetsBy(assets, field) {
    let total = 0.0
    for (let key in assets) {
        if (assets[key]) {
            let a = assets[key]
            let value = a[field]
            total += value
        }
    }

    return total
}

const useStyles = makeStyles((theme) => ({
    paper: {
        marginTop: theme.spacing(3),
        marginBottom: theme.spacing(3),
        padding: theme.spacing(2),
    },
    root: {
        minWidth: 275,
    },
    bullet: {
        display: 'inline-block',
        margin: '0 2px',
        transform: 'scale(0.8)',
    },
    title: {
        fontSize: 14,
    },
    pos: {
        marginTop: 12,
    },
}));

export default function TotalAssetSummary() {
    const classes = useStyles();
    const [assets, setAssets] = useState({})

    useEffect(() => {
        queryAssets((assets) => {
            setAssets(assets)
        })
    }, [])

    return <Card className={classes.root} variant="outlined">
        <CardContent>
            <Typography className={classes.title} color="textSecondary" gutterBottom>
                Total Account Balance
            </Typography>
            <Typography variant="h5" component="h2">
                {Math.round(aggregateAssetsBy(assets, "inBTC") * 1e8) / 1e8} <span>BTC</span>
            </Typography>

            <Typography className={classes.pos} color="textSecondary">
                Estimated Value
            </Typography>

            <Typography variant="h5" component="h3">
                {Math.round(aggregateAssetsBy(assets, "inUSD") * 100) / 100} <span>USD</span>
            </Typography>
        </CardContent>
    </Card>
}
