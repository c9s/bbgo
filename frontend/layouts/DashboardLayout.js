import React from "react";

import {makeStyles} from "@material-ui/core/styles";
import AppBar from "@material-ui/core/AppBar";
import Toolbar from "@material-ui/core/Toolbar";
import Typography from "@material-ui/core/Typography";
import Container from '@material-ui/core/Container';

import SideBar from "../components/SideBar";

import ConnectWallet from '../components/ConnectWallet';

const useStyles = makeStyles((theme) => ({
    root: {
        flexGrow: 1,
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
    container: { },
    toolbar:{
        justifyContent: 'space-between',
    }
}));

export default function DashboardLayout({children}) {
    const classes = useStyles();

    return (
        <div className={classes.root}>
            <AppBar className={classes.appBar}>
                <Toolbar className={classes.toolbar}>
                    <Typography variant="h6" className={classes.title}>
                        BBGO
                    </Typography>
                    {/* <Button color="inherit">Login</Button> */}
                    <ConnectWallet />
                </Toolbar>
            </AppBar>

            <SideBar/>

            <main className={classes.content}>
                <div className={classes.appBarSpacer}/>
                <Container className={classes.container} maxWidth={false} disableGutters={true}>
                    {children}
                </Container>
            </main>
        </div>
    );
}
