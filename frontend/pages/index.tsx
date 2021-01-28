import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Grid from '@material-ui/core/Grid';
import Paper from '@material-ui/core/Paper';

import TotalAssetsPie from '../components/TotalAssetsPie';
import TotalAssetSummary from '../components/TotalAssetsSummary';

import TradingVolumePanel from '../components/TradingVolumePanel';
import ExchangeSessionTabPanel from '../components/ExchangeSessionTabPanel';

const useStyles = makeStyles((theme) => ({
    paper: {
        height: 140,
        width: 200,
    },
    totalAssetsBox: {
        height: 300,
    },
    totalAssetsSummary: {},
    TradingVolumeBar: {
        height: 400,
    },
    control: {
        padding: theme.spacing(2),
    },
}));


export default function Home() {
    const classes = useStyles();

    return (
        <Container>
            <Paper className={classes.totalAssetsSummary}>
                <Box m={4}>
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
                </Box>
            </Paper>

            <TradingVolumePanel/>

            <ExchangeSessionTabPanel/>
        </Container>
    );
}

