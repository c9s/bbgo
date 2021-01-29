import React, {useEffect, useState} from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Paper from '@material-ui/core/Paper';
import {queryTrades} from '../api/bbgo';
import {DataGrid} from '@material-ui/data-grid';

const columns = [
    {field: 'gid', headerName: 'GID', width: 80, type: 'number'},
    {field: 'exchange', headerName: 'Exchange'},
    {field: 'symbol', headerName: 'Symbol'},
    {field: 'side', headerName: 'Side', width: 90},
    {field: 'price', headerName: 'Price', type: 'number', width: 120 },
    {field: 'quantity', headerName: 'Quantity', type: 'number'},
    {field: 'isMargin', headerName: 'Margin'},
    {field: 'isIsolated', headerName: 'Isolated'},
    {field: 'tradedAt', headerName: 'Trade Time', width: 200},
];

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
    },
}));

export default function Trades() {
    const classes = useStyles();

    const [trades, setTrades] = useState([])

    useEffect(() => {
        queryTrades({}, (trades) => {
            setTrades(trades.map((o) => { o.id = o.gid; return o }))
        })
    }, [])

    return (
        <Container>
            <Box m={4}>
                <Paper className={classes.paper}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Trades
                    </Typography>
                </Paper>
                <DataGrid
                    rows={trades}
                    columns={columns}
                    pageSize={50}
                    autoHeight={true}/>
            </Box>
        </Container>
    );
}

