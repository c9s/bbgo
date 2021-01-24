import React from 'react';

import { makeStyles } from '@material-ui/core/styles';
import Container from '@material-ui/core/Container';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Link from '@material-ui/core/Link';
import Grid from '@material-ui/core/Grid';
import Paper from '@material-ui/core/Paper';

import styles from '../styles/Home.module.css'

const useStyles = makeStyles((theme) => ({
  root: {
    flexGrow: 1,
  },
  paper: {
    height: 140,
    width: 100,
  },
  control: {
    padding: theme.spacing(2),
  },
}));


export default function Home() {
  const classes = useStyles();

  return (
    <Container maxWidth="sm">
      <Box my={4}>
        <Grid container className={classes.root}>
          <Grid item xs={12}>
            <Grid container justify="center">
              <Grid item>
                 <Paper className={classes.paper}/>
              </Grid>
            </Grid>
          </Grid>
        </Grid>

        <Typography variant="h4" component="h1" gutterBottom>
          Next.js example
        </Typography>

        <Link href="/about" color="secondary">
          Go to the about page
        </Link>
      </Box>
    </Container>
  );
}

