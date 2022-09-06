import React from 'react';
import Grid from '@mui/material/Grid';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import TextField from '@mui/material/TextField';
import FormHelperText from '@mui/material/FormHelperText';
import Radio from '@mui/material/Radio';
import RadioGroup from '@mui/material/RadioGroup';
import FormControlLabel from '@mui/material/FormControlLabel';
import FormControl from '@mui/material/FormControl';
import FormLabel from '@mui/material/FormLabel';

import Alert from '@mui/lab/Alert';

import { configureDatabase, testDatabaseConnection } from '../api/bbgo';

import { makeStyles } from '@mui/styles';

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

export default function ConfigureDatabaseForm({ onConfigured }) {
  const classes = useStyles();

  const [mysqlURL, setMysqlURL] = React.useState(
    'root@tcp(127.0.0.1:3306)/bbgo'
  );

  const [driver, setDriver] = React.useState('sqlite3');
  const [testing, setTesting] = React.useState(false);
  const [testResponse, setTestResponse] = React.useState(null);
  const [configured, setConfigured] = React.useState(false);

  const getDSN = () => (driver === 'sqlite3' ? 'file:bbgo.sqlite3' : mysqlURL);

  const resetTestResponse = () => {
    setTestResponse(null);
  };

  const handleConfigureDatabase = (event) => {
    const dsn = getDSN();

    configureDatabase({ driver, dsn }, (response) => {
      console.log(response);
      setTesting(false);
      setTestResponse(response);
      if (onConfigured) {
        setConfigured(true);
        setTimeout(onConfigured, 3000);
      }
    }).catch((err) => {
      console.error(err);
      setTesting(false);
      setTestResponse(err.response.data);
    });
  };

  const handleTestConnection = (event) => {
    const dsn = getDSN();

    setTesting(true);
    testDatabaseConnection({ driver, dsn }, (response) => {
      console.log(response);
      setTesting(false);
      setTestResponse(response);
    }).catch((err) => {
      console.error(err);
      setTesting(false);
      setTestResponse(err.response.data);
    });
  };

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Configure Database
      </Typography>

      <Typography variant="body1" gutterBottom>
        If you have database installed on your machine, you can enter the DSN
        string in the following field. Please note this is optional, you CAN
        SKIP this step.
      </Typography>

      <Grid container spacing={3}>
        <Grid item xs={12} sm={4}>
          <Box m={6}>
            <FormControl component="fieldset" required={true}>
              <FormLabel component="legend">Database Driver</FormLabel>
              <RadioGroup
                aria-label="driver"
                name="driver"
                value={driver}
                onChange={(event) => {
                  setDriver(event.target.value);
                }}
              >
                <FormControlLabel
                  value="sqlite3"
                  control={<Radio />}
                  label="Standard (Default)"
                />
                <FormControlLabel
                  value="mysql"
                  control={<Radio />}
                  label="MySQL"
                />
              </RadioGroup>
            </FormControl>
            <FormHelperText></FormHelperText>
          </Box>
        </Grid>

        {driver === 'mysql' ? (
          <Grid item xs={12} sm={8}>
            <TextField
              id="mysql_url"
              name="mysql_url"
              label="MySQL Data Source Name"
              fullWidth
              required
              defaultValue={mysqlURL}
              onChange={(event) => {
                setMysqlURL(event.target.value);
                resetTestResponse();
              }}
            />
            <FormHelperText>MySQL DSN</FormHelperText>

            <Typography variant="body1" gutterBottom>
              If you have database installed on your machine, you can enter the
              DSN string like the following format:
              <br />
              <pre>
                <code>root:password@tcp(127.0.0.1:3306)/bbgo</code>
              </pre>
              <br />
              Be sure to create your database before using it. You need to
              execute the following statement to create a database:
              <br />
              <pre>
                <code>CREATE DATABASE bbgo CHARSET utf8;</code>
              </pre>
            </Typography>
          </Grid>
        ) : (
          <Grid item xs={12} sm={8}>
            <Box m={6}>
              <Typography variant="body1" gutterBottom>
                If you don't know what to choose, just pick the standard driver
                (sqlite3).
                <br />
                For professionals, you can pick MySQL driver, BBGO works best
                with MySQL, especially for larger data scale.
              </Typography>
            </Box>
          </Grid>
        )}
      </Grid>

      <div className={classes.buttons}>
        <Button
          color="primary"
          onClick={handleTestConnection}
          disabled={testing || configured}
        >
          {testing ? 'Testing' : 'Test Connection'}
        </Button>

        <Button
          variant="contained"
          color="primary"
          disabled={testing || configured}
          onClick={handleConfigureDatabase}
        >
          Configure
        </Button>
      </div>

      {testResponse ? (
        testResponse.error ? (
          <Box m={2}>
            <Alert severity="error">{testResponse.error}</Alert>
          </Box>
        ) : testResponse.success ? (
          <Box m={2}>
            <Alert severity="success">Connection Test Succeeded</Alert>
          </Box>
        ) : null
      ) : null}
    </React.Fragment>
  );
}
