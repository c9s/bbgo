import React, { useEffect, useState } from 'react';

import { makeStyles } from '@mui/styles';
import Typography from '@mui/material/Typography';
import Paper from '@mui/material/Paper';
import PlainLayout from '../../layouts/PlainLayout';
import { QRCodeSVG } from 'qrcode.react';
import { queryOutboundIP } from '../../api/bbgo';

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
  },
}));

function fetchConnectUrl(cb) {
  return queryOutboundIP((outboundIP) => {
    cb(
      window.location.protocol + '//' + outboundIP + ':' + window.location.port
    );
  });
}

export default function Connect() {
  const classes = useStyles();

  const [connectUrl, setConnectUrl] = useState([]);

  useEffect(() => {
    fetchConnectUrl(function (url) {
      setConnectUrl(url);
    });
  }, []);

  return (
    <PlainLayout title={'Connect'}>
      <Paper className={classes.paper}>
        <Typography variant="h4" gutterBottom>
          Sign In Using QR Codes
        </Typography>
        <div className={classes.dataGridContainer}>
          <QRCodeSVG size={160} style={{ flexGrow: 1 }} value={connectUrl} />
        </div>
      </Paper>
    </PlainLayout>
  );
}
