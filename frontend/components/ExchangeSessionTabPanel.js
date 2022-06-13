import Paper from '@mui/material/Paper';
import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import React, { useEffect, useState } from 'react';
import { querySessions } from '../api/bbgo';
import Typography from '@mui/material/Typography';
import { makeStyles } from '@mui/styles';

const useStyles = makeStyles((theme) => ({
  paper: {
    margin: theme.spacing(2),
    padding: theme.spacing(2),
  },
}));

export default function ExchangeSessionTabPanel() {
  const classes = useStyles();

  const [tabIndex, setTabIndex] = React.useState(0);
  const handleTabClick = (event, newValue) => {
    setTabIndex(newValue);
  };

  const [sessions, setSessions] = useState([]);

  useEffect(() => {
    querySessions((sessions) => {
      setSessions(sessions);
    });
  }, []);

  return (
    <Paper className={classes.paper}>
      <Typography variant="h4" gutterBottom>
        Sessions
      </Typography>
      <Tabs
        value={tabIndex}
        onChange={handleTabClick}
        indicatorColor="primary"
        textColor="primary"
      >
        {sessions.map((session) => {
          return <Tab key={session.name} label={session.name} />;
        })}
      </Tabs>
    </Paper>
  );
}
