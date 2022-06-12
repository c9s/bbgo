import React from 'react';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import List from '@mui/material/List';
import Card from '@mui/material/Card';
import CardHeader from '@mui/material/CardHeader';
import CardContent from '@mui/material/CardContent';
import Avatar from '@mui/material/Avatar';
import IconButton from '@mui/material/IconButton';
import MoreVertIcon from '@mui/icons-material/MoreVert';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';

import { makeStyles } from '@mui/styles';
import { queryStrategies } from '../api/bbgo';

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
    },
  },
}));

function configToTable(config) {
  const rows = Object.getOwnPropertyNames(config).map((k) => {
    return {
      key: k,
      val: config[k],
    };
  });

  return (
    <TableContainer>
      <Table aria-label="strategy attributes">
        <TableHead>
          <TableRow>
            <TableCell>Field</TableCell>
            <TableCell align="right">Value</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.key}>
              <TableCell component="th" scope="row">
                {row.key}
              </TableCell>
              <TableCell align="right">{row.val}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

export default function ReviewStrategies({ onBack, onNext }) {
  const classes = useStyles();

  const [strategies, setStrategies] = React.useState([]);

  React.useEffect(() => {
    queryStrategies((strategies) => {
      setStrategies(strategies || []);
    }).catch((err) => {
      console.error(err);
    });
  }, []);

  const items = strategies.map((o, i) => {
    const mounts = o.on || [];
    delete o.on;

    const config = o[o.strategy];

    const titleComps = [o.strategy.toUpperCase()];
    if (config.symbol) {
      titleComps.push(config.symbol);
    }

    const title = titleComps.join(' ');

    return (
      <Card key={i} className={classes.strategyCard}>
        <CardHeader
          avatar={<Avatar aria-label="strategy">G</Avatar>}
          action={
            <IconButton aria-label="settings">
              <MoreVertIcon />
            </IconButton>
          }
          title={title}
          subheader={`Exchange ${mounts.map((m) => m.toUpperCase())}`}
        />
        <CardContent>
          <Typography variant="body2" color="textSecondary" component="p">
            Strategy will be executed on session {mounts.join(',')} with the
            following configuration:
          </Typography>

          {configToTable(config)}
        </CardContent>
      </Card>
    );
  });

  return (
    <React.Fragment>
      <Typography variant="h6" gutterBottom>
        Review Strategies
      </Typography>

      <List component="nav">{items}</List>

      <div className={classes.buttons}>
        <Button
          onClick={() => {
            if (onBack) {
              onBack();
            }
          }}
        >
          Add New Strategy
        </Button>

        <Button
          variant="contained"
          color="primary"
          onClick={() => {
            if (onNext) {
              onNext();
            }
          }}
        >
          Next
        </Button>
      </div>
    </React.Fragment>
  );
}
