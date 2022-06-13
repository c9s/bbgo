import React from 'react';
import { useRouter } from 'next/router';

import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { makeStyles } from '@mui/styles';

import { ping, saveConfig, setupRestart } from '../api/bbgo';
import Box from '@mui/material/Box';
import Alert from '@mui/lab/Alert';

const useStyles = makeStyles((theme) => ({
  strategyCard: {
    margin: theme.spacing(1),
  },
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

export default function SaveConfigAndRestart({ onBack, onRestarted }) {
  const classes = useStyles();

  const { push } = useRouter();
  const [response, setResponse] = React.useState({});

  const handleRestart = () => {
    saveConfig((resp) => {
      setResponse(resp);

      setupRestart((resp) => {
        let t;
        t = setInterval(() => {
          ping(() => {
            clearInterval(t);
            push('/');
          });
        }, 1000);
      }).catch((err) => {
        console.error(err);
        setResponse(err.response.data);
      });

      // call restart here
    }).catch((err) => {
      console.error(err);
      setResponse(err.response.data);
    });
  };

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Save Config and Restart
      </Typography>

      <Typography variant="body1" gutterBottom>
        Click "Save and Restart" to save the configurations to the config file{' '}
        <code>bbgo.yaml</code>, and save the exchange session credentials to the
        dotenv file <code>.env.local</code>.
      </Typography>

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

        <Button variant="contained" color="primary" onClick={handleRestart}>
          Save and Restart
        </Button>
      </div>

      {response ? (
        response.error ? (
          <Box m={2}>
            <Alert severity="error">{response.error}</Alert>
          </Box>
        ) : response.success ? (
          <Box m={2}>
            <Alert severity="success">Config Saved</Alert>
          </Box>
        ) : null
      ) : null}
    </React.Fragment>
  );
}
