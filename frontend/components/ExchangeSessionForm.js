import React from 'react';
import Grid from '@material-ui/core/Grid';
import Typography from '@material-ui/core/Typography';
import TextField from '@material-ui/core/TextField';
import FormControlLabel from '@material-ui/core/FormControlLabel';
import InputLabel from '@material-ui/core/InputLabel';
import FormControl from '@material-ui/core/FormControl';

import Checkbox from '@material-ui/core/Checkbox';
import Select from '@material-ui/core/Select';
import MenuItem from '@material-ui/core/MenuItem';

import {makeStyles} from '@material-ui/core/styles';

const useStyles = makeStyles((theme) => ({
    formControl: {
        marginTop: theme.spacing(1),
        marginBottom: theme.spacing(1),
        minWidth: 120,
    },
}));

export default function ExchangeSessionForm() {

    const classes = useStyles();
    const [exchangeType, setExchangeType] = React.useState('max');


    const [isMargin, setIsMargin] = React.useState(false);
    const [isIsolatedMargin, setIsIsolatedMargin] = React.useState(false);
    const [isolatedMarginSymbol, setIsolatedMarginSymbol] = React.useState("");

    const handleExchangeTypeChange = (event) => {
        setExchangeType(event.target.value);
    };

    const handleIsMarginChange = (event) => {
        setIsMargin(event.target.checked);
    };

    const handleIsIsolatedMarginChange = (event) => {
        setIsIsolatedMargin(event.target.checked);
    };

    return (
        <React.Fragment>
            <Typography variant="h6" gutterBottom>
                Add Exchange Session
            </Typography>
            <Grid container spacing={3}>

                <Grid item xs={12}>
                    <FormControl className={classes.formControl}>
                        <InputLabel id="exchange-type-select-label">Exchange Type</InputLabel>
                        <Select
                            labelId="exchange-type-select-label"
                            id="exchange-type-select"
                            value={exchangeType}
                            onChange={handleExchangeTypeChange}
                        >
                            <MenuItem value={"binance"}>Binance</MenuItem>
                            <MenuItem value={"max"}>Max</MenuItem>
                        </Select>
                    </FormControl>
                </Grid>

                <Grid item xs={12} sm={12}>
                    <TextField
                        required
                        id="name"
                        name="name"
                        label="Session Name"
                        fullWidth
                        autoComplete="given-name"
                    />
                </Grid>

                <Grid item xs={12} sm={6}>
                    <TextField id="state" name="state" label="State/Province/Region" fullWidth/>
                </Grid>
                <Grid item xs={12} sm={6}>
                    <TextField
                        required
                        id="zip"
                        name="zip"
                        label="Zip / Postal code"
                        fullWidth
                        autoComplete="shipping postal-code"
                    />
                </Grid>
                <Grid item xs={12} sm={6}>
                    <TextField
                        required
                        id="country"
                        name="country"
                        label="Country"
                        fullWidth
                        autoComplete="shipping country"
                    />
                </Grid>

                <Grid item xs={12}>
                    <FormControlLabel
                        control={<Checkbox color="secondary" name="isMargin" onChange={handleIsMarginChange}
                                           value="1"/>}
                        label="Use margin trading. This is only available for Binance"
                    />
                </Grid>

                <Grid item xs={12}>
                    <FormControlLabel
                        control={<Checkbox color="secondary" name="isIsolatedMargin"
                                           onChange={handleIsIsolatedMarginChange} value="1"/>}
                        label="Use isolated margin trading, if this is set, you can only trade one symbol with one session. This is only available for Binance"
                    />
                </Grid>

                {isIsolatedMargin ?
                    <Grid item xs={12}>
                        <TextField
                            required
                            id="isolatedMarginSymbol"
                            name="isolatedMarginSymbol"
                            label="Isolated Margin Symbol"
                            fullWidth
                        />
                    </Grid>
                    : null}


            </Grid>
        </React.Fragment>
    );
}
