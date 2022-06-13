import React, { useEffect, useState } from 'react';

import { makeStyles } from '@mui/styles';
import Typography from '@mui/material/Typography';
import Paper from '@mui/material/Paper';
import { queryClosedOrders } from '../api/bbgo';
import { DataGrid } from '@mui/x-data-grid';
import DashboardLayout from '../layouts/DashboardLayout';

const columns = [
  { field: 'gid', headerName: 'GID', width: 80, type: 'number' },
  { field: 'clientOrderID', headerName: 'Client Order ID', width: 130 },
  { field: 'exchange', headerName: 'Exchange' },
  { field: 'symbol', headerName: 'Symbol' },
  { field: 'orderType', headerName: 'Type' },
  { field: 'side', headerName: 'Side', width: 90 },
  {
    field: 'averagePrice',
    headerName: 'Average Price',
    type: 'number',
    width: 120,
  },
  { field: 'quantity', headerName: 'Quantity', type: 'number' },
  {
    field: 'executedQuantity',
    headerName: 'Executed Quantity',
    type: 'number',
  },
  { field: 'status', headerName: 'Status' },
  { field: 'isMargin', headerName: 'Margin' },
  { field: 'isIsolated', headerName: 'Isolated' },
  { field: 'creationTime', headerName: 'Create Time', width: 200 },
];

const useStyles = makeStyles((theme) => ({
  paper: {
    margin: theme.spacing(2),
    padding: theme.spacing(2),
  },
  dataGridContainer: {
    display: 'flex',
    height: 'calc(100vh - 64px - 120px)',
  },
}));

export default function Orders() {
  const classes = useStyles();

  const [orders, setOrders] = useState([]);

  useEffect(() => {
    queryClosedOrders({}, (orders) => {
      setOrders(
        orders.map((o) => {
          o.id = o.gid;
          return o;
        })
      );
    });
  }, []);

  return (
    <DashboardLayout>
      <Paper className={classes.paper}>
        <Typography variant="h4" gutterBottom>
          Orders
        </Typography>
        <div className={classes.dataGridContainer}>
          <div style={{ flexGrow: 1 }}>
            <DataGrid
              rows={orders}
              columns={columns}
              pageSize={50}
              autoPageSize={true}
            />
          </div>
        </div>
      </Paper>
    </DashboardLayout>
  );
}
