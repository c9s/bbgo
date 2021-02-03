import React from 'react';
import Grid from '@material-ui/core/Grid';
import Box from '@material-ui/core/Box';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import TextField from '@material-ui/core/TextField';
import FormHelperText from '@material-ui/core/FormHelperText';

import Alert from '@material-ui/lab/Alert';

import {testDatabaseConnection, configureDatabase} from '../api/bbgo';

import {makeStyles} from '@material-ui/core/styles';

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

export default function ConfigureDatabaseForm({ onConfigured }) {
    const classes = useStyles();

    const [mysqlURL, setMysqlURL] = React.useState("root@tcp(127.0.0.1:3306)/bbgo")
    const [testing, setTesting] = React.useState(false);
    const [testResponse, setTestResponse] = React.useState(null);
    const [configured, setConfigured] = React.useState(false);

    const resetTestResponse = () => {
        setTestResponse(null)
    }

    const handleConfigureDatabase = (event) => {
        configureDatabase(mysqlURL, (response) => {
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
        })
    }

    const handleTestConnection = (event) => {
        setTesting(true);
        testDatabaseConnection(mysqlURL, (response) => {
            console.log(response)
            setTesting(false)
            setTestResponse(response)
        }).catch((err) => {
            console.error(err)
            setTesting(false)
            setTestResponse(err.response.data)
        })
    };

    return (
        <React.Fragment>
            <Typography variant="h6" gutterBottom>
                Configure Database
            </Typography>

            <Typography variant="body1" gutterBottom>
            If you have database installed on your machine, you can enter the DSN string in the following field.
            Please note this is optional, you CAN SKIP this step.
            </Typography>

            <Grid container spacing={3}>
                <Grid item xs={12} sm={6}>
                    <TextField id="mysql_url" name="mysql_url" label="MySQL Data Source Name"
                               fullWidth
                               required
                               defaultValue={mysqlURL}
                               onChange={(event) => {
                                   setMysqlURL(event.target.value)
                                   resetTestResponse()
                               }}
                    />

                    <FormHelperText id="session-name-helper-text">
                        If you have database installed on your machine, you can enter the DSN string like the
                            following
                            format:
                        <br/>
                        <code>
                            root:password@tcp(127.0.0.1:3306)/bbgo
                        </code>

                        <br/>
                        Be sure to create your database before using it. You need to execute the following statement
                            to
                            create a database:
                        <br/>
                        <code>
                            CREATE DATABASE bbgo CHARSET utf8;
                        </code>

                    </FormHelperText>
                </Grid>
            </Grid>

            <div className={classes.buttons}>
                <Button
                    color="primary"
                    onClick={handleTestConnection}
                    disabled={testing || configured}
                >
                    {testing ? "Testing" : "Test Connection"}
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

            {
                testResponse ? testResponse.error ? (
                    <Box m={2}>
                        <Alert severity="error">{testResponse.error}</Alert>
                    </Box>
                ) : testResponse.success ? (
                    <Box m={2}>
                        <Alert severity="success">Connection Test Succeeded</Alert>
                    </Box>
                ) : null : null
            }


        </React.Fragment>
    );

}
