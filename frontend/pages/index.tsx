import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Link from '@material-ui/core/Link';
import Grid from '@material-ui/core/Grid';
import Paper from '@material-ui/core/Paper';
import Tabs from '@material-ui/core/Tabs';
import Tab from '@material-ui/core/Tab';

import TotalAssetsPie from '../components/TotalAssetsPie';
import TotalAssetSummary from '../components/TotalAssetsSummary';

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

            <ExchangeSessionTabPanel/>
        </Container>
    );
}

