import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Grid from '@material-ui/core/Grid';
import Paper from '@material-ui/core/Paper';

import TotalAssetsPie from '../components/TotalAssetsPie';
import TotalAssetSummary from '../components/TotalAssetsSummary';

import TradingVolumePanel from '../components/TradingVolumePanel';
import ExchangeSessionTabPanel from '../components/ExchangeSessionTabPanel';

import DashboardLayout from '../layouts/DashboardLayout';

const useStyles = makeStyles((theme) => ({
    paper: {
        height: 140,
        width: 200,
    },
    totalAssetsBox: {
        height: 300,
    },
    totalAssetsSummary: {
        padding: theme.spacing(2),
    },
    control: {
        padding: theme.spacing(2),
    },
}));


// props are pageProps passed from _app.tsx
export default function Home(props) {
    const classes = useStyles();

    return (
        <DashboardLayout>
            <Box m={4}>
                <Paper className={classes.totalAssetsSummary}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Total Assets
                    </Typography>

                    <Grid container spacing={3}>
                        <Grid item xs={12} md={6}>
                            <TotalAssetSummary/>
                        </Grid>

                        <Grid item xs={12} md={6}>
                            <Box className={classes.totalAssetsBox}>
                                <TotalAssetsPie/>
                            </Box>
                        </Grid>
                    </Grid>
                </Paper>

                <TradingVolumePanel/>

                <ExchangeSessionTabPanel/>
            </Box>
        </DashboardLayout>
    );
}

