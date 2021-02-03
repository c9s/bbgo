import React from 'react';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import List from '@material-ui/core/List';
import Card from '@material-ui/core/Card';
import CardHeader from '@material-ui/core/CardHeader';
import CardContent from '@material-ui/core/CardContent';
import Avatar from '@material-ui/core/Avatar';
import IconButton from '@material-ui/core/IconButton';
import MoreVertIcon from '@material-ui/icons/MoreVert';
import Table from '@material-ui/core/Table';
import TableBody from '@material-ui/core/TableBody';
import TableCell from '@material-ui/core/TableCell';
import TableContainer from '@material-ui/core/TableContainer';
import TableHead from '@material-ui/core/TableHead';
import TableRow from '@material-ui/core/TableRow';

import {makeStyles} from '@material-ui/core/styles';

import {saveConfig} from "../api/bbgo";

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
        }
    },
}));

export default function SaveConfigAndRestart({onBack, onRestarted}) {
    const classes = useStyles();

    const [strategies, setStrategies] = React.useState([]);

    const [response, setResponse] = React.useState({});

    React.useEffect(() => {
    }, [])

    const handleRestart = () => {
        saveConfig((resp) => {
            setResponse(resp);

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
                Click "Save and Restart" to save the configurations to the config file <code>bbgo.yaml</code>,
                and save the exchange session credentials to the dotenv file <code>.env.local</code>.
            </Typography>

            <div className={classes.buttons}>
                <Button onClick={() => {
                    if (onBack) {
                        onBack()
                    }
                }}>
                    Back
                </Button>

                <Button
                    variant="contained"
                    color="primary"
                    onClick={handleRestart}>
                    Save and Restart
                </Button>
            </div>
        </React.Fragment>
    );
}
