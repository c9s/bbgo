import React, {useEffect, useState} from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Paper from '@material-ui/core/Paper';
import PlainLayout from '../../layouts/PlainLayout';
import {QRCodeSVG} from 'qrcode.react';

const useStyles = makeStyles((theme) => ({
    paper: {
        margin: theme.spacing(2),
        padding: theme.spacing(2),
    },
    dataGridContainer: {
        display: 'flex',
        textAlign: 'center',
        alignItems: 'center',
        alignContent: 'center',
        height: 320,
    }
}));

function fetchConnectUrl(cb) {
    cb(window.location.protocol + "//" + window.location.host)
}

export default function Connect() {
    const classes = useStyles();

    const [connectUrl, setConnectUrl] = useState([])

    useEffect(() => {
        fetchConnectUrl(function (url) {
            setConnectUrl(url)
        })
    }, [])

    return (
        <PlainLayout title={"Connect"}>
            <Paper className={classes.paper}>
                <Typography variant="h4" gutterBottom>
                    Sign In Using QR Codes
                </Typography>
                <div className={classes.dataGridContainer}>
                    <QRCodeSVG
                        size={160}
                        style={{flexGrow: 1}}
                        value={connectUrl}/>
                </div>
            </Paper>
        </PlainLayout>
    );
}

