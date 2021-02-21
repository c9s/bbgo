import React from 'react';
import PropTypes from 'prop-types';
import Head from 'next/head';

import {ThemeProvider} from '@material-ui/core/styles';

import Dialog from '@material-ui/core/Dialog';
import DialogContent from '@material-ui/core/DialogContent';
import DialogContentText from '@material-ui/core/DialogContentText';
import DialogTitle from '@material-ui/core/DialogTitle';
import LinearProgress from '@material-ui/core/LinearProgress';
import Box from '@material-ui/core/Box';

import CssBaseline from '@material-ui/core/CssBaseline';
import theme from '../src/theme';
import '../styles/globals.css'
import {querySyncStatus} from "../api/bbgo";

export default function MyApp(props) {
    const {Component, pageProps} = props;

    const [syncing, setSyncing] = React.useState(true)

    React.useEffect(() => {
        // Remove the server-side injected CSS.
        const jssStyles = document.querySelector('#jss-server-side');
        if (jssStyles) {
            jssStyles.parentElement.removeChild(jssStyles);
        }

        let poller = null
        const pollSyncStatus = () => {
            querySyncStatus((status) => {
                setSyncing(status)
                if (!status) {
                    clearInterval(poller)
                }
            }).catch((err) => {
                console.error(err)
            })
        }
        poller = setInterval(pollSyncStatus, 1000)


    }, []);

    const handleClose = (e) => {}

    return (
        <React.Fragment>
            <Head>
                <title>BBGO</title>
                <meta name="viewport" content="minimum-scale=1, initial-scale=1, width=device-width"/>
            </Head>
            <ThemeProvider theme={theme}>
                {/* CssBaseline kickstart an elegant, consistent, and simple baseline to build upon. */}
                <CssBaseline/>
                {
                    syncing ? (
                        <React.Fragment>
                            <Dialog
                                open={syncing}
                                aria-labelledby="alert-dialog-title"
                                aria-describedby="alert-dialog-description"
                            >
                                <DialogTitle id="alert-dialog-title">{"Syncing Trades"}</DialogTitle>
                                <DialogContent>
                                    <DialogContentText id="alert-dialog-description">
                                        The environment is syncing trades from the exchange sessions.
                                        Please wait a moment...
                                    </DialogContentText>
                                    <Box m={2}>
                                        <LinearProgress />
                                    </Box>
                                </DialogContent>
                            </Dialog>
                        </React.Fragment>
                    ) : (
                        <Component {...pageProps}/>
                    )
                }
            </ThemeProvider>
        </React.Fragment>
    );
}

MyApp.propTypes = {
    Component: PropTypes.elementType.isRequired,
    pageProps: PropTypes.object.isRequired,
};
