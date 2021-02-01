import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Paper from '@material-ui/core/Paper';

import PlainLayout from '../../layouts/PlainLayout';

const useStyles = makeStyles((theme) => ({
    paper: {
        height: 140,
    },
    control: {
        padding: theme.spacing(2),
    },
}));

export default function SetupSession() {
    const classes = useStyles();
    return (
        <PlainLayout>
            <Box m={4}>
                <Paper className={classes.paper}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Setup Session
                    </Typography>
                </Paper>
            </Box>
        </PlainLayout>
    );
}

