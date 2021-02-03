import React from 'react';
import Grid from '@material-ui/core/Grid';
import Button from '@material-ui/core/Button';
import Typography from '@material-ui/core/Typography';
import List from '@material-ui/core/List';
import ListItem from '@material-ui/core/ListItem';
import ListItemText from '@material-ui/core/ListItemText';
import ListItemIcon from '@material-ui/core/ListItemIcon';
import AndroidIcon from '@material-ui/icons/Android';
import Card from '@material-ui/core/Card';
import CardHeader from '@material-ui/core/CardHeader';
import CardContent from '@material-ui/core/CardContent';
import Avatar from '@material-ui/core/Avatar';
import IconButton from '@material-ui/core/IconButton';
import MoreVertIcon from '@material-ui/icons/MoreVert';
import ExpandMoreIcon from '@material-ui/icons/ExpandMore';
import Table from '@material-ui/core/Table';
import TableBody from '@material-ui/core/TableBody';
import TableCell from '@material-ui/core/TableCell';
import TableContainer from '@material-ui/core/TableContainer';
import TableHead from '@material-ui/core/TableHead';
import TableRow from '@material-ui/core/TableRow';
import CardActions from '@material-ui/core/CardActions';

import {makeStyles} from '@material-ui/core/styles';
import {querySessions, queryStrategies} from "../api/bbgo";

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
        }
    },
}));

function configToTable(config) {
    const rows = Object.getOwnPropertyNames(config).map((k) => {
        return {
            key: k,
            val: config[k],
        }
    })

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

export default function ReviewStrategies({onBack, onNext}) {
    const classes = useStyles();

    const [strategies, setStrategies] = React.useState([]);

    React.useEffect(() => {
        queryStrategies((strategies) => {
            setStrategies(strategies || [])
        }).catch((err) => {
            console.error(err);
        });
    }, [])

    const items = strategies.map((o, i) => {
        const mounts = o.on || [];
        delete o.on

        const config = o[o.strategy]

        const titleComps = [o.strategy.toUpperCase()]
        if (config.symbol) {
            titleComps.push(config.symbol)
        }

        const title = titleComps.join(" ")

        return (
            <Card key={i} className={classes.strategyCard}>
                <CardHeader
                    avatar={
                        <Avatar aria-label="strategy">G</Avatar>
                    }
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
                        Strategy will be executed on session {mounts.join(',')} with the following configuration:
                    </Typography>

                    {configToTable(config)}

                </CardContent>
            </Card>
        );

        return (
            <ListItem key={i}>
                <ListItemIcon>
                    <AndroidIcon/>
                </ListItemIcon>
                <ListItemText primary={o.strategy + desc} secondary={mounts}/>
            </ListItem>
        );
    })

    return (
        <React.Fragment>
            <Typography variant="h6" gutterBottom>
                Review Strategies
            </Typography>

            <List component="nav">
                {items}
            </List>

            <div className={classes.buttons}>
                <Button onClick={() => { if (onBack) { onBack() } }}>
                    Add New Strategy
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
