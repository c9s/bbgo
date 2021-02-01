import React, {useEffect, useState} from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Paper from '@material-ui/core/Paper';
import {queryClosedOrders} from '../api/bbgo';
import {DataGrid} from '@material-ui/data-grid';
import DashboardLayout from '../layouts/DashboardLayout';


const columns = [
    {field: 'gid', headerName: 'GID', width: 80, type: 'number'},
    {field: 'clientOrderID', headerName: 'Client Order ID', width: 130},
    {field: 'exchange', headerName: 'Exchange'},
    {field: 'symbol', headerName: 'Symbol'},
    {field: 'orderType', headerName: 'Type'},
    {field: 'side', headerName: 'Side', width: 90},
    {field: 'averagePrice', headerName: 'Average Price', type: 'number', width: 120},
    {field: 'quantity', headerName: 'Quantity', type: 'number'},
    {field: 'executedQuantity', headerName: 'Executed Quantity', type: 'number'},
    {field: 'status', headerName: 'Status'},
    {field: 'isMargin', headerName: 'Margin'},
    {field: 'isIsolated', headerName: 'Isolated'},
    {field: 'creationTime', headerName: 'Create Time', width: 200},
];

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
    },
}));

export default function Orders() {
    const classes = useStyles();

    const [orders, setOrders] = useState([])

    useEffect(() => {
        queryClosedOrders({}, (orders) => {
            setOrders(orders.map((o) => {
                o.id = o.gid;
                return o
            }))
        })
    }, [])

    return (
        <DashboardLayout>
            <Box m={4}>
                <Paper className={classes.paper}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Orders
                    </Typography>
                </Paper>
                <DataGrid
                    rows={orders}
                    columns={columns}
                    pageSize={50}
                    autoHeight={true}/>
            </Box>
        </DashboardLayout>
    );
}

