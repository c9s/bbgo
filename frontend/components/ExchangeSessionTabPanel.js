import Paper from "@material-ui/core/Paper";
import Box from "@material-ui/core/Box";
import Tabs from "@material-ui/core/Tabs";
import Tab from "@material-ui/core/Tab";
import Link from "@material-ui/core/Link";
import React, {useEffect, useState} from "react";
import {querySessions} from '../api/bbgo'

export default function ExchangeSessionTabPanel() {
    const [tabIndex, setTabIndex] = React.useState(0);
    const handleTabClick = (event, newValue) => {
        setTabIndex(newValue);
    };

    const [sessions, setSessions] = useState({})

    useEffect(() => {
        querySessions((sessions) => {
            setSessions(sessions)
        })
    }, [])

    return <Paper>
        <Box m={4}>
            <Tabs
                value={tabIndex}
                onChange={handleTabClick}
                indicatorColor="primary"
                textColor="primary"
            >
                <Tab label="Item One"/>
                <Tab label="Item Two"/>
                <Tab label="Item Three"/>
            </Tabs>

            <Link href="/about" color="secondary">
                Go to the about page
            </Link>
        </Box>
    </Paper>
}
