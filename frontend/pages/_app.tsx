import React from 'react';
import PropTypes from 'prop-types';
import Head from 'next/head';

import {makeStyles, ThemeProvider} from '@material-ui/core/styles';
import AppBar from '@material-ui/core/AppBar';
import Toolbar from '@material-ui/core/Toolbar';
import Typography from '@material-ui/core/Typography';

import SideBar from '../components/SideBar';

import CssBaseline from '@material-ui/core/CssBaseline';
import theme from '../src/theme';
import '../styles/globals.css'

const drawerWidth = 240;

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
    toolbar: {
        paddingRight: 24, // keep right padding when drawer closed
    },
    toolbarIcon: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        padding: '0 8px',
        ...theme.mixins.toolbar,
    },
    title: {
        flexGrow: 1,
    },
    appBar: {
        zIndex: theme.zIndex.drawer + 1,
    },
    appBarSpacer: theme.mixins.toolbar,
}));

export default function MyApp(props) {
    const {Component, pageProps} = props;

    const classes = useStyles();

    React.useEffect(() => {
        // Remove the server-side injected CSS.
        const jssStyles = document.querySelector('#jss-server-side');
        if (jssStyles) {
            jssStyles.parentElement.removeChild(jssStyles);
        }
    }, []);

    return (
        <React.Fragment>
            <Head>
                <title>BBGO</title>
                <meta name="viewport" content="minimum-scale=1, initial-scale=1, width=device-width"/>
            </Head>
            <ThemeProvider theme={theme}>
                {/* CssBaseline kickstart an elegant, consistent, and simple baseline to build upon. */}
                <CssBaseline/>

                <div className={classes.root}>
                    <AppBar className={classes.appBar}>
                        <Toolbar>
                            <Typography variant="h6" className={classes.title}>
                                BBGO
                            </Typography>
                            {/* <Button color="inherit">Login</Button> */}
                        </Toolbar>
                    </AppBar>

                    <SideBar/>

                    <main className={classes.content}>
                        <div className={classes.appBarSpacer}/>
                        <Component {...pageProps} />
                    </main>
                </div>

            </ThemeProvider>
        </React.Fragment>
    );
}

MyApp.propTypes = {
    Component: PropTypes.elementType.isRequired,
    pageProps: PropTypes.object.isRequired,
};
