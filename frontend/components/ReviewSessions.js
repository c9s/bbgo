import React from 'react';
import Grid from '@mui/core/Grid';
import Button from '@mui/core/Button';
import Typography from '@mui/core/Typography';
import List from '@mui/core/List';
import ListItem from '@mui/core/ListItem';
import ListItemText from '@mui/core/ListItemText';
import ListItemIcon from '@mui/core/ListItemIcon';
import PowerIcon from '@mui/icons/Power';

import { makeStyles } from '@mui/core/styles';
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
