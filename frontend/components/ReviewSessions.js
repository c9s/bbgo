import React from 'react';
import Grid from '@mui/material/Grid';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemText from '@mui/material/ListItemText';
import ListItemIcon from '@mui/material/ListItemIcon';
import PowerIcon from '@mui/icons-material/Power';

import { makeStyles } from '@mui/styles';
import { querySessions } from '../api/bbgo';

const useStyles = makeStyles((theme) => ({
  formControl: {
    marginTop: theme.spacing(1),
    marginBottom: theme.spacing(1),
    minWidth: 120,
  },
  buttons: {
    display: 'flex',
    justifyContent: 'flex-end',
    marginTop: theme.spacing(2),
    paddingTop: theme.spacing(2),
    paddingBottom: theme.spacing(2),
    '& > *': {
      marginLeft: theme.spacing(1),
    },
  },
}));

export default function ReviewSessions({ onBack, onNext }) {
  const classes = useStyles();

  const [sessions, setSessions] = React.useState([]);

  React.useEffect(() => {
    querySessions((sessions) => {
      setSessions(sessions);
    });
  }, []);

  const items = sessions.map((session, i) => {
    console.log(session);
    return (
      <ListItem key={session.name}>
        <ListItemIcon>
          <PowerIcon />
        </ListItemIcon>
        <ListItemText primary={session.name} secondary={session.exchange} />
      </ListItem>
    );
  });

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Review Sessions
      </Typography>

      <List component="nav">{items}</List>

      <div className={classes.buttons}>
        <Button
          onClick={() => {
            if (onBack) {
              onBack();
            }
          }}
        >
          Back
        </Button>

        <Button
          variant="contained"
          color="primary"
          onClick={() => {
            if (onNext) {
              onNext();
            }
          }}
        >
          Next
        </Button>
      </div>
    </React.Fragment>
  );
}
