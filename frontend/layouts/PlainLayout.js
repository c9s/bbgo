import React from "react";

import {makeStyles} from "@material-ui/core/styles";
import AppBar from "@material-ui/core/AppBar";
import Toolbar from "@material-ui/core/Toolbar";
import Typography from "@material-ui/core/Typography";
import Container from '@material-ui/core/Container';

const useStyles = makeStyles((theme) => ({
    root: {
        // flexGrow: 1,
        display: 'flex',
    },
    content: {
        flexGrow: 1,
        height: '100vh',
        overflow: 'auto',
    },
    appBar: {
        zIndex: theme.zIndex.drawer + 1,
    },
    appBarSpacer: theme.mixins.toolbar,
}));

export default function PlainLayout(props) {
    const classes = useStyles();
    return <div className={classes.root}>
        <AppBar className={classes.appBar}>
            <Toolbar>
                <Typography variant="h6" className={classes.title}>
                    { props && props.title ? props.title : "BBGO Setup Wizard" }
                </Typography>
            </Toolbar>
        </AppBar>

        <main className={classes.content}>
            <div className={classes.appBarSpacer}/>
            <Container>
                {props.children}
            </Container>
        </main>
    </div>;
}
