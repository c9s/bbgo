import Paper from "@material-ui/core/Paper";
import Box from "@material-ui/core/Box";
import Tabs from "@material-ui/core/Tabs";
import Tab from "@material-ui/core/Tab";
import React from "react";
import TradingVolumeBar from "./TradingVolumeBar";
import {makeStyles} from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Typography from "@material-ui/core/Typography";

const useStyles = makeStyles((theme) => ({
    tradingVolumeBarBox: {
        height: 400,
    },
    paper: {
        marginTop: theme.spacing(3),
        marginBottom: theme.spacing(3),
        padding: theme.spacing(2),
    }
}));

export default function TradingVolumePanel() {
    const [period, setPeriod] = React.useState("day");
    const [segment, setSegment] = React.useState("exchange");
    const classes = useStyles();
    const handlePeriodChange = (event, newValue) => {
        setPeriod(newValue);
    };

    const handleSegmentChange = (event, newValue) => {
        setSegment(newValue);
    };

    return <Paper className={classes.paper}>
        <Typography variant="h4" gutterBottom>
            Trading Volume
        </Typography>

        <Grid container spacing={0}>
            <Grid item xs={12} md={6}>
                <Tabs value={period}
                      onChange={handlePeriodChange}
                      indicatorColor="primary"
                      textColor="primary">
                    <Tab label="Day" value={"day"}/>
                    <Tab label="Month" value={"month"}/>
                    <Tab label="Year" value={"year"}/>
                </Tabs>
            </Grid>
            <Grid item xs={12} md={6}>
                <Grid container justify={"flex-end"}>
                    <Tabs value={segment}
                          onChange={handleSegmentChange}
                          indicatorColor="primary"
                          textColor="primary">
                        <Tab label="By Exchange" value={"exchange"}/>
                        <Tab label="By Symbol" value={"symbol"}/>
                    </Tabs>
                </Grid>
            </Grid>
        </Grid>

        <Box className={classes.tradingVolumeBarBox}>
            <TradingVolumeBar period={period} segment={segment}/>
        </Box>
    </Paper>;
}
