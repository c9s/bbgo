import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Paper from '@material-ui/core/Paper';

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
    },
}));

export default function Orders() {
    const classes = useStyles();

    return (
        <Container>
            <Box m={4}>
                <Paper className={classes.paper}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Orders
                    </Typography>
                </Paper>
            </Box>
        </Container>
    );
}

