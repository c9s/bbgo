import React from 'react';
import Grid from '@material-ui/core/Grid';
import Box from '@material-ui/core/Box';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import TextField from '@material-ui/core/TextField';
import FormControlLabel from '@material-ui/core/FormControlLabel';
import FormHelperText from '@material-ui/core/FormHelperText';
import InputLabel from '@material-ui/core/InputLabel';
import FormControl from '@material-ui/core/FormControl';
import InputAdornment from '@material-ui/core/InputAdornment';
import IconButton from '@material-ui/core/IconButton';

import Checkbox from '@material-ui/core/Checkbox';
import Select from '@material-ui/core/Select';
import MenuItem from '@material-ui/core/MenuItem';
import FilledInput from '@material-ui/core/FilledInput';

import Alert from '@material-ui/lab/Alert';
import VisibilityOff from '@material-ui/icons/VisibilityOff';
import Visibility from '@material-ui/icons/Visibility';

import { addSession, testSessionConnection } from '../api/bbgo';

import { makeStyles } from '@material-ui/core/styles';

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

export default function AddExchangeSessionForm({ onBack, onAdded }) {
  const classes = useStyles();
  const [exchangeType, setExchangeType] = React.useState('max');
  const [customSessionName, setCustomSessionName] = React.useState(false);
  const [sessionName, setSessionName] = React.useState(exchangeType);

  const [testing, setTesting] = React.useState(false);
  const [testResponse, setTestResponse] = React.useState(null);
  const [response, setResponse] = React.useState(null);

  const [apiKey, setApiKey] = React.useState('');
  const [apiSecret, setApiSecret] = React.useState('');

  const [showApiKey, setShowApiKey] = React.useState(false);
  const [showApiSecret, setShowApiSecret] = React.useState(false);

  const [isMargin, setIsMargin] = React.useState(false);
  const [isIsolatedMargin, setIsIsolatedMargin] = React.useState(false);
  const [isolatedMarginSymbol, setIsolatedMarginSymbol] = React.useState('');

  const resetTestResponse = () => {
    setTestResponse(null);
  };

  const handleExchangeTypeChange = (event) => {
    setExchangeType(event.target.value);
    setSessionName(event.target.value);
    resetTestResponse();
  };

  const createSessionConfig = () => {
    return {
      name: sessionName,
      exchange: exchangeType,
      key: apiKey,
      secret: apiSecret,
      margin: isMargin,
      envVarPrefix: exchangeType.toUpperCase(),
      isolatedMargin: isIsolatedMargin,
      isolatedMarginSymbol: isolatedMarginSymbol,
    };
  };

  const handleAdd = (event) => {
    const payload = createSessionConfig();
    addSession(payload, (response) => {
      setResponse(response);
      if (onAdded) {
        setTimeout(onAdded, 3000);
      }
    }).catch((error) => {
      console.error(error);
      setResponse(error.response);
    });
  };

  const handleTestConnection = (event) => {
    const payload = createSessionConfig();
    setTesting(true);
    testSessionConnection(payload, (response) => {
      console.log(response);
      setTesting(false);
      setTestResponse(response);
    }).catch((error) => {
      console.error(error);
      setTesting(false);
      setTestResponse(error.response);
    });
  };

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Add Exchange Session
      </Typography>
      <Grid container spacing={3}>
        <Grid item xs={12}>
          <FormControl className={classes.formControl}>
            <InputLabel id="exchange-type-select-label">Exchange</InputLabel>
            <Select
              labelId="exchange-type-select-label"
              id="exchange-type-select"
              value={exchangeType}
              onChange={handleExchangeTypeChange}
            >
              <MenuItem value={'binance'}>Binance</MenuItem>
              <MenuItem value={'max'}>Max</MenuItem>
            </Select>
          </FormControl>
        </Grid>

        <Grid item xs={12} sm={6}>
          <TextField
            id="name"
            name="name"
            label="Session Name"
            fullWidth
            required
            disabled={!customSessionName}
            onChange={(event) => {
              setSessionName(event.target.value);
            }}
            value={sessionName}
          />
        </Grid>

        <Grid item xs={12} sm={6}>
          <FormControlLabel
            control={
              <Checkbox
                color="secondary"
                name="custom_session_name"
                onChange={(event) => {
                  setCustomSessionName(event.target.checked);
                }}
                value="1"
              />
            }
            label="Custom exchange session name"
          />
          <FormHelperText id="session-name-helper-text">
            By default, the session name will be the exchange type name, e.g.{' '}
            <code>binance</code> or <code>max</code>.<br />
            If you're using multiple exchange sessions, you might need to custom
            the session name. <br />
            This is for advanced users.
          </FormHelperText>
        </Grid>

        <Grid item xs={12}>
          <FormControl fullWidth variant="filled">
            <InputLabel htmlFor="apiKey">API Key</InputLabel>
            <FilledInput
              id="apiKey"
              type={showApiKey ? 'text' : 'password'}
              value={apiKey}
              endAdornment={
                <InputAdornment position="end">
                  <IconButton
                    aria-label="toggle key visibility"
                    onClick={() => {
                      setShowApiKey(!showApiKey);
                    }}
                    onMouseDown={(event) => {
                      event.preventDefault();
                    }}
                    edge="end"
                  >
                    {showApiKey ? <Visibility /> : <VisibilityOff />}
                  </IconButton>
                </InputAdornment>
              }
              onChange={(event) => {
                setApiKey(event.target.value);
                resetTestResponse();
              }}
            />
          </FormControl>
        </Grid>

        <Grid item xs={12}>
          <FormControl fullWidth variant="filled">
            <InputLabel htmlFor="apiSecret">API Secret</InputLabel>
            <FilledInput
              id="apiSecret"
              type={showApiSecret ? 'text' : 'password'}
              value={apiSecret}
              endAdornment={
                <InputAdornment position="end">
                  <IconButton
                    aria-label="toggle key visibility"
                    onClick={() => {
                      setShowApiSecret(!showApiSecret);
                    }}
                    onMouseDown={(event) => {
                      event.preventDefault();
                    }}
                    edge="end"
                  >
                    {showApiSecret ? <Visibility /> : <VisibilityOff />}
                  </IconButton>
                </InputAdornment>
              }
              onChange={(event) => {
                setApiSecret(event.target.value);
                resetTestResponse();
              }}
            />
          </FormControl>
        </Grid>

        {exchangeType === 'binance' ? (
          <Grid item xs={12}>
            <FormControlLabel
              control={
                <Checkbox
                  color="secondary"
                  name="isMargin"
                  onChange={(event) => {
                    setIsMargin(event.target.checked);
                    resetTestResponse();
                  }}
                  value="1"
                />
              }
              label="Use margin trading."
            />
            <FormHelperText id="isMargin-helper-text">
              This is only available for Binance. Please use the leverage at
              your own risk.
            </FormHelperText>

            <FormControlLabel
              control={
                <Checkbox
                  color="secondary"
                  name="isIsolatedMargin"
                  onChange={(event) => {
                    setIsIsolatedMargin(event.target.checked);
                    resetTestResponse();
                  }}
                  value="1"
                />
              }
              label="Use isolated margin trading."
            />
            <FormHelperText id="isIsolatedMargin-helper-text">
              This is only available for Binance. If this is set, you can only
              trade one symbol with one session.
            </FormHelperText>

            {isIsolatedMargin ? (
              <TextField
                id="isolatedMarginSymbol"
                name="isolatedMarginSymbol"
                label="Isolated Margin Symbol"
                onChange={(event) => {
                  setIsolatedMarginSymbol(event.target.value);
                  resetTestResponse();
                }}
                fullWidth
                required
              />
            ) : null}
          </Grid>
        ) : null}
      </Grid>

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
          color="primary"
          onClick={handleTestConnection}
          disabled={testing}
        >
          {testing ? 'Testing' : 'Test Connection'}
        </Button>

        <Button variant="contained" color="primary" onClick={handleAdd}>
          Add
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

      {response ? (
        response.error ? (
          <Box m={2}>
            <Alert severity="error">{response.error}</Alert>
          </Box>
        ) : response.success ? (
          <Box m={2}>
            <Alert severity="success">Exchange Session Added</Alert>
          </Box>
        ) : null
      ) : null}
    </React.Fragment>
  );
}
