import Paper from "@material-ui/core/Paper";
import Tabs from "@material-ui/core/Tabs";
import Tab from "@material-ui/core/Tab";
import React, {useEffect, useState} from "react";
import {querySessions} from '../api/bbgo'
import Typography from "@material-ui/core/Typography";
import {makeStyles} from "@material-ui/core/styles";

const useStyles = makeStyles((theme) => ({
    paper: {
        marginTop: theme.spacing(3),
        marginBottom: theme.spacing(3),
        padding: theme.spacing(2),
    }
}));

export default function ExchangeSessionTabPanel() {
    const classes = useStyles();

    const [tabIndex, setTabIndex] = React.useState(0);
    const handleTabClick = (event, newValue) => {
        setTabIndex(newValue);
    };

    const [sessions, setSessions] = useState([])

    useEffect(() => {
        querySessions((sessions) => {
            setSessions(sessions)
        })
    }, [])

    return <Paper className={classes.paper}>
        <Typography variant="h4" gutterBottom>
            Sessions
        </Typography>
        <Tabs
            value={tabIndex}
            onChange={handleTabClick}
            indicatorColor="primary"
            textColor="primary"
        >
            {
                sessions.map((session) => {
                    return <Tab key={session.name} label={session.name}/>
                })
            }
        </Tabs>
    </Paper>
}
