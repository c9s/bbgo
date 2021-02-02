import React from 'react';
import Grid from '@material-ui/core/Grid';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import List from '@material-ui/core/List';
import ListItem from '@material-ui/core/ListItem';
import ListItemText from '@material-ui/core/ListItemText';
import ListItemIcon from '@material-ui/core/ListItemIcon';
import PowerIcon from '@material-ui/icons/Power';

import {makeStyles} from '@material-ui/core/styles';
import {querySessions} from "../api/bbgo";
import {Power} from "@material-ui/icons";

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

export default function ReviewSessions({onBack, onNext}) {
    const classes = useStyles();

    const [sessions, setSessions] = React.useState([]);

    React.useEffect(() => {
        querySessions((sessions) => {
            setSessions(sessions)
        });
    }, [])

    const items = sessions.map((session, i) => {
        console.log(session)
        return (
            <ListItem key={session.name}>
                <ListItemIcon>
                    <PowerIcon/>
                </ListItemIcon>
                <ListItemText primary={session.name} secondary={session.exchange}/>
            </ListItem>
        );
    })

    return (
        <React.Fragment>
            <Typography variant="h6" gutterBottom>
                Review Sessions
            </Typography>

            <List component="nav">
                {items}
            </List>

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
                    onClick={() => {
                        if (onNext) {
                            onNext();
                        }
                    }}>
                    Next
                </Button>
            </div>
        </React.Fragment>
    );
}
