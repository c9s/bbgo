import React from 'react';

import {makeStyles} from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Box from '@material-ui/core/Box';
import Paper from '@material-ui/core/Paper';
import Stepper from '@material-ui/core/Stepper';
import Step from '@material-ui/core/Step';
import StepLabel from '@material-ui/core/StepLabel';

import ExchangeSessionForm from "../../components/ExchangeSessionForm";
import ExchangeSessionReview from "../../components/ExchangeSessionReview";
import ExchangeSessionTest from "../../components/ExchangeSessionTest";

import PlainLayout from '../../layouts/PlainLayout';

const useStyles = makeStyles((theme) => ({
    paper: {
        padding: theme.spacing(2),
    },
}));

const steps = ['Add Exchange Session', 'Review Settings', 'Test Connection'];

function getStepContent(step) {
    switch (step) {
        case 0:
            return <ExchangeSessionForm/>;
        case 1:
            return <ExchangeSessionReview/>;
        case 2:
            return <ExchangeSessionTest/>;
        default:
            throw new Error('Unknown step');
    }
}

export default function SetupSession() {
    const classes = useStyles();
    const [activeStep, setActiveStep] = React.useState(0);

    return (
        <PlainLayout>
            <Box m={4}>
                <Paper className={classes.paper}>
                    <Typography variant="h4" component="h2" gutterBottom>
                        Setup Session
                    </Typography>

                    <Stepper activeStep={activeStep} className={classes.stepper}>
                        {steps.map((label) => (
                            <Step key={label}>
                                <StepLabel>{label}</StepLabel>
                            </Step>
                        ))}
                    </Stepper>

                    <React.Fragment>
                        {getStepContent(activeStep)}
                    </React.Fragment>
                </Paper>
            </Box>
        </PlainLayout>
    );
}

