import React from 'react';
import PropTypes from 'prop-types';

import Grid from '@material-ui/core/Grid';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';

import {makeStyles} from '@material-ui/core/styles';
import {querySessions, querySessionSymbols} from "../api/bbgo";

import TextField from '@material-ui/core/TextField';
import FormControlLabel from '@material-ui/core/FormControlLabel';
import FormHelperText from '@material-ui/core/FormHelperText';
import InputLabel from '@material-ui/core/InputLabel';
import FormControl from '@material-ui/core/FormControl';

import Checkbox from '@material-ui/core/Checkbox';
import Select from '@material-ui/core/Select';
import MenuItem from '@material-ui/core/MenuItem';

import Alert from '@material-ui/lab/Alert';
import Box from "@material-ui/core/Box";

import NumberFormat from 'react-number-format';

function NumberFormatCustom(props) {
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

NumberFormatCustom.propTypes = {
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
        }
    },
}));


export default function ConfigureGridStrategyForm({onBack, onConfigured}) {
    const classes = useStyles();

    const [sessions, setSessions] = React.useState([]);

    const [activeSessionSymbols, setActiveSessionSymbols] = React.useState([]);

    const [selectedSessionName, setSelectedSessionName] = React.useState(null);

    const [selectedSymbol, setSelectedSymbol] = React.useState('');

    const [upperPrice, setUpperPrice] = React.useState(30000.0);
    const [lowerPrice, setLowerPrice] = React.useState(10000.0);

    const [response, setResponse] = React.useState({});

    React.useEffect(() => {
        querySessions((sessions) => {
            setSessions(sessions)
        });
    }, [])

    const handleAdd = (event) => {

    };

    const handleSessionChange = (event) => {
        const sessionName = event.target.value;
        setSelectedSessionName(sessionName)

        querySessionSymbols(sessionName, (symbols) => {
            setActiveSessionSymbols(symbols);
        })
    };

    const sessionMenuItems = sessions.map((session, index) => {
        return (
            <MenuItem key={session.name} value={session.name}>
                {session.name}
            </MenuItem>
        );
    })

    const symbolMenuItems = activeSessionSymbols.map((symbol, index) => {
        return (
            <MenuItem key={symbol} value={symbol}>
                {symbol}
            </MenuItem>
        );
    })

    return (
        <React.Fragment>
            <Typography variant="h6" gutterBottom>
                Add Grid Strategy
            </Typography>

            <Typography variant="body1" gutterBottom>
                Fixed price band grid strategy uses the fixed price band to place buy/sell orders.
                This strategy places sell orders above the current price, places buy orders below the current price.
                If any of the order is executed, then it will automatically place a new profit order on the reverse
                side.
            </Typography>

            <Grid container spacing={3}>
                <Grid item xs={12}>
                    <FormControl className={classes.formControl}>
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
                    <FormControl className={classes.formControl}>
                        <InputLabel id="symbol-select-label">Market</InputLabel>
                        <Select
                            labelId="symbol-select-label"
                            id="symbol-select"
                            value={selectedSymbol ? selectedSymbol : ''}
                            onChange={(event) => {
                                setSelectedSymbol(event.target.value);
                            }}>
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
                            if (event.target.value) {
                                const v = parseFloat(event.target.value)
                                if (!isNaN(v)) {
                                    setUpperPrice(v);
                                }
                            }
                        }}
                        value={upperPrice}
                        InputProps={{
                            inputComponent: NumberFormatCustom,
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
                            if (event.target.value) {
                                const v = parseFloat(event.target.value)
                                if (!isNaN(v)) {
                                    setLowerPrice(v);
                                }
                            }
                        }}
                        value={lowerPrice}
                        InputProps={{
                            inputComponent: NumberFormatCustom,
                        }}
                    />
                </Grid>

                <Grid item xs={12} sm={6}>
                    <FormControlLabel
                        control={<Checkbox color="secondary" name="custom_session_name"
                                           onChange={(event) => {

                                           }} value="1"/>}
                        label="Custom exchange session name"
                    />
                    <FormHelperText id="session-name-helper-text">
                        By default, the session name will be the exchange type name,
                        e.g. <code>binance</code> or <code>max</code>.<br/>
                        If you're using multiple exchange sessions, you might need to custom the session name. <br/>
                        This is for advanced users.
                    </FormHelperText>
                </Grid>

                <Grid item xs={12}>
                    <TextField id="key" name="api_key" label="API Key"
                               fullWidth
                               required
                               onChange={(event) => {
                               }}
                    />
                </Grid>

                <Grid item xs={12}>
                    <TextField id="secret" name="api_secret" label="API Secret"
                               fullWidth
                               required
                               onChange={(event) => {
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
                    }}>
                    Back
                </Button>

                <Button
                    variant="contained"
                    color="primary"
                    onClick={handleAdd}
                >
                    Add
                </Button>
            </div>

            {
                response ? response.error ? (
                    <Box m={2}>
                        <Alert severity="error">{response.error}</Alert>
                    </Box>
                ) : response.success ? (
                    <Box m={2}>
                        <Alert severity="success">Exchange Session Added</Alert>
                    </Box>
                ) : null : null
            }


        </React.Fragment>
    );
}
