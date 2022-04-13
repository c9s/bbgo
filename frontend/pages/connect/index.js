import React, {useEffect, useState} from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Paper from '@material-ui/core/Paper';
import PlainLayout from '../../layouts/PlainLayout';

const useStyles = makeStyles((theme) => ({
    paper: {
        margin: theme.spacing(2),
        padding: theme.spacing(2),
    },
    dataGridContainer: {
        display: 'flex',
        height: 'calc(100vh - 64px - 120px)',
    }
}));

export default function Orders() {
    const classes = useStyles();

    const [orders, setOrders] = useState([])

    useEffect(() => {
    }, [])

    return (
        <PlainLayout>
            <Paper className={classes.paper}>
                <Typography variant="h4" gutterBottom>
                    Sign In Using QR Codes
                </Typography>
                <div className={classes.dataGridContainer}>
                    <div style={{flexGrow: 1}}>
                        QRCode
                    </div>
                </div>
            </Paper>
        </PlainLayout>
    );
}

