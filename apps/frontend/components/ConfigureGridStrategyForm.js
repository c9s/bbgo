import React from 'react';
import PropTypes from 'prop-types';

import Grid from '@mui/material/Grid';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';

import { makeStyles } from '@mui/styles';
import {
  attachStrategyOn,
  querySessions,
  querySessionSymbols,
} from '../api/bbgo';

import TextField from '@mui/material/TextField';
import FormControlLabel from '@mui/material/FormControlLabel';
import FormHelperText from '@mui/material/FormHelperText';
import InputLabel from '@mui/material/InputLabel';
import FormControl from '@mui/material/FormControl';
import Radio from '@mui/material/Radio';
import RadioGroup from '@mui/material/RadioGroup';
import FormLabel from '@mui/material/FormLabel';
import Select from '@mui/material/Select';
import MenuItem from '@mui/material/MenuItem';

import Alert from '@mui/lab/Alert';
import Box from '@mui/material/Box';

import NumberFormat from 'react-number-format';

function parseFloatValid(s) {
  if (s) {
    const f = parseFloat(s);
    if (!isNaN(f)) {
      return f;
    }
  }

  return null;
}

function parseFloatCall(s, cb) {
  if (s) {
    const f = parseFloat(s);
    if (!isNaN(f)) {
      cb(f);
    }
  }
}

function StandardNumberFormat(props) {
  const { inputRef, onChange, ...other } = props;
  return (
    <NumberFormat
      {...other}
      getInputRef={inputRef}
      onValueChange={(values) => {
        onChange({
          target: {
            name: props.name,
            value: values.value,
          },
        });
      }}
      thousandSeparator
      isNumericString
    />
  );
}

StandardNumberFormat.propTypes = {
  inputRef: PropTypes.func.isRequired,
  name: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired,
};

function PriceNumberFormat(props) {
  const { inputRef, onChange, ...other } = props;

  return (
    <NumberFormat
      {...other}
      getInputRef={inputRef}
      onValueChange={(values) => {
        onChange({
          target: {
            name: props.name,
            value: values.value,
          },
        });
      }}
      thousandSeparator
      isNumericString
      prefix="$"
    />
  );
}

PriceNumberFormat.propTypes = {
  inputRef: PropTypes.func.isRequired,
  name: PropTypes.string.isRequired,
  onChange: PropTypes.func.isRequired,
};

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

export default function ConfigureGridStrategyForm({ onBack, onAdded }) {
  const classes = useStyles();

  const [errors, setErrors] = React.useState({});

  const [sessions, setSessions] = React.useState([]);

  const [activeSessionSymbols, setActiveSessionSymbols] = React.useState([]);

  const [selectedSessionName, setSelectedSessionName] = React.useState(null);

  const [selectedSymbol, setSelectedSymbol] = React.useState('');

  const [quantityBy, setQuantityBy] = React.useState('fixedAmount');

  const [upperPrice, setUpperPrice] = React.useState(30000.0);
  const [lowerPrice, setLowerPrice] = React.useState(10000.0);

  const [fixedAmount, setFixedAmount] = React.useState(100.0);
  const [fixedQuantity, setFixedQuantity] = React.useState(1.234);
  const [gridNumber, setGridNumber] = React.useState(20);
  const [profitSpread, setProfitSpread] = React.useState(100.0);

  const [response, setResponse] = React.useState({});

  React.useEffect(() => {
    querySessions((sessions) => {
      setSessions(sessions);
    });
  }, []);

  const handleAdd = (event) => {
    const payload = {
      symbol: selectedSymbol,
      gridNumber: parseFloatValid(gridNumber),
      profitSpread: parseFloatValid(profitSpread),
      upperPrice: parseFloatValid(upperPrice),
      lowerPrice: parseFloatValid(lowerPrice),
    };
    switch (quantityBy) {
      case 'fixedQuantity':
        payload.quantity = parseFloatValid(fixedQuantity);
        break;

      case 'fixedAmount':
        payload.amount = parseFloatValid(fixedAmount);
        break;
    }

    if (!selectedSessionName) {
      setErrors({ session: true });
      return;
    }

    if (!selectedSymbol) {
      setErrors({ symbol: true });
      return;
    }

    console.log(payload);
    attachStrategyOn(selectedSessionName, 'grid', payload, (response) => {
      console.log(response);
      setResponse(response);
      if (onAdded) {
        setTimeout(onAdded, 3000);
      }
    })
      .catch((err) => {
        console.error(err);
        setResponse(err.response.data);
      })
      .finally(() => {
        setErrors({});
      });
  };

  const handleQuantityBy = (event) => {
    setQuantityBy(event.target.value);
  };

  const handleSessionChange = (event) => {
    const sessionName = event.target.value;
    setSelectedSessionName(sessionName);

    querySessionSymbols(sessionName, (symbols) => {
      setActiveSessionSymbols(symbols);
    }).catch((err) => {
      console.error(err);
      setResponse(err.response.data);
    });
  };

  const sessionMenuItems = sessions.map((session, index) => {
    return (
      <MenuItem key={session.name} value={session.name}>
        {session.name}
      </MenuItem>
    );
  });

  const symbolMenuItems = activeSessionSymbols.map((symbol, index) => {
    return (
      <MenuItem key={symbol} value={symbol}>
        {symbol}
      </MenuItem>
    );
  });

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Add Grid Strategy
      </Typography>

      <Typography variant="body1" gutterBottom>
        Fixed price band grid strategy uses the fixed price band to place
        buy/sell orders. This strategy places sell orders above the current
        price, places buy orders below the current price. If any of the order is
        executed, then it will automatically place a new profit order on the
        reverse side.
      </Typography>

      <Grid container spacing={3}>
        <Grid item xs={12}>
          <FormControl
            required
            className={classes.formControl}
            error={errors.session}
          >
            <InputLabel id="session-select-label">Session</InputLabel>
            <Select
              labelId="session-select-label"
              id="session-select"
              value={selectedSessionName ? selectedSessionName : ''}
              onChange={handleSessionChange}
            >
              {sessionMenuItems}
            </Select>
          </FormControl>
          <FormHelperText id="session-select-helper-text">
            Select the exchange session you want to mount this strategy.
          </FormHelperText>
        </Grid>

        <Grid item xs={12}>
          <FormControl
            required
            className={classes.formControl}
            error={errors.symbol}
          >
            <InputLabel id="symbol-select-label">Market</InputLabel>
            <Select
              labelId="symbol-select-label"
              id="symbol-select"
              value={selectedSymbol ? selectedSymbol : ''}
              onChange={(event) => {
                setSelectedSymbol(event.target.value);
              }}
            >
              {symbolMenuItems}
            </Select>
          </FormControl>
          <FormHelperText id="session-select-helper-text">
            Select the market you want to run this strategy
          </FormHelperText>
        </Grid>

        <Grid item xs={12}>
          <TextField
            id="upperPrice"
            name="upper_price"
            label="Upper Price"
            fullWidth
            required
            onChange={(event) => {
              parseFloatCall(event.target.value, setUpperPrice);
            }}
            value={upperPrice}
            InputProps={{
              inputComponent: PriceNumberFormat,
            }}
          />
        </Grid>

        <Grid item xs={12}>
          <TextField
            id="lowerPrice"
            name="lower_price"
            label="Lower Price"
            fullWidth
            required
            onChange={(event) => {
              parseFloatCall(event.target.value, setLowerPrice);
            }}
            value={lowerPrice}
            InputProps={{
              inputComponent: PriceNumberFormat,
            }}
          />
        </Grid>

        <Grid item xs={12}>
          <TextField
            id="profitSpread"
            name="profit_spread"
            label="Profit Spread"
            fullWidth
            required
            onChange={(event) => {
              parseFloatCall(event.target.value, setProfitSpread);
            }}
            value={profitSpread}
            InputProps={{
              inputComponent: StandardNumberFormat,
            }}
          />
        </Grid>

        <Grid item xs={12} sm={3}>
          <FormControl component="fieldset">
            <FormLabel component="legend">Order Quantity By</FormLabel>
            <RadioGroup
              name="quantityBy"
              value={quantityBy}
              onChange={handleQuantityBy}
            >
              <FormControlLabel
                value="fixedAmount"
                control={<Radio />}
                label="Fixed Amount"
              />
              <FormControlLabel
                value="fixedQuantity"
                control={<Radio />}
                label="Fixed Quantity"
              />
            </RadioGroup>
          </FormControl>
        </Grid>

        <Grid item xs={12} sm={9}>
          {quantityBy === 'fixedQuantity' ? (
            <TextField
              id="fixedQuantity"
              name="order_quantity"
              label="Fixed Quantity"
              fullWidth
              required
              onChange={(event) => {
                parseFloatCall(event.target.value, setFixedQuantity);
              }}
              value={fixedQuantity}
              InputProps={{
                inputComponent: StandardNumberFormat,
              }}
            />
          ) : null}

          {quantityBy === 'fixedAmount' ? (
            <TextField
              id="orderAmount"
              name="order_amount"
              label="Fixed Amount"
              fullWidth
              required
              onChange={(event) => {
                parseFloatCall(event.target.value, setFixedAmount);
              }}
              value={fixedAmount}
              InputProps={{
                inputComponent: PriceNumberFormat,
              }}
            />
          ) : null}
        </Grid>

        <Grid item xs={12}>
          <TextField
            id="gridNumber"
            name="grid_number"
            label="Number of Grid"
            fullWidth
            required
            onChange={(event) => {
              parseFloatCall(event.target.value, setGridNumber);
            }}
            value={gridNumber}
            InputProps={{
              inputComponent: StandardNumberFormat,
            }}
          />
        </Grid>
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

        <Button variant="contained" color="primary" onClick={handleAdd}>
          Add Strategy
        </Button>
      </div>

      {response ? (
        response.error ? (
          <Box m={2}>
            <Alert severity="error">{response.error}</Alert>
          </Box>
        ) : response.success ? (
          <Box m={2}>
            <Alert severity="success">Strategy Added</Alert>
          </Box>
        ) : null
      ) : null}
    </React.Fragment>
  );
}
