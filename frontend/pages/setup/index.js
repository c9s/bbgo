import React from 'react';

import { makeStyles } from '@mui/styles';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import Paper from '@mui/material/Paper';
import Stepper from '@mui/material/Stepper';
import Step from '@mui/material/Step';
import StepLabel from '@mui/material/StepLabel';

import ConfigureDatabaseForm from '../../components/ConfigureDatabaseForm';
import AddExchangeSessionForm from '../../components/AddExchangeSessionForm';
import ReviewSessions from '../../components/ReviewSessions';
import ConfigureGridStrategyForm from '../../components/ConfigureGridStrategyForm';
import ReviewStrategies from '../../components/ReviewStrategies';
import SaveConfigAndRestart from '../../components/SaveConfigAndRestart';

import PlainLayout from '../../layouts/PlainLayout';

const useStyles = makeStyles((theme) => ({
  paper: {
    padding: theme.spacing(2),
  },
}));

const steps = [
  'Configure Database',
  'Add Exchange Session',
  'Review Sessions',
  'Configure Strategy',
  'Review Strategies',
  'Save Config and Restart',
];

function getStepContent(step, setActiveStep) {
  switch (step) {
    case 0:
      return (
        <ConfigureDatabaseForm
          onConfigured={() => {
            setActiveStep(1);
          }}
        />
      );
    case 1:
      return (
        <AddExchangeSessionForm
          onBack={() => {
            setActiveStep(0);
          }}
          onAdded={() => {
            setActiveStep(2);
          }}
        />
      );
    case 2:
      return (
        <ReviewSessions
          onBack={() => {
            setActiveStep(1);
          }}
          onNext={() => {
            setActiveStep(3);
          }}
        />
      );
    case 3:
      return (
        <ConfigureGridStrategyForm
          onBack={() => {
            setActiveStep(2);
          }}
          onAdded={() => {
            setActiveStep(4);
          }}
        />
      );
    case 4:
      return (
        <ReviewStrategies
          onBack={() => {
            setActiveStep(3);
          }}
          onNext={() => {
            setActiveStep(5);
          }}
        />
      );

    case 5:
      return (
        <SaveConfigAndRestart
          onBack={() => {
            setActiveStep(4);
          }}
          onRestarted={() => {}}
        />
      );

    default:
      throw new Error('Unknown step');
  }
}

export default function Setup() {
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
            {getStepContent(activeStep, setActiveStep)}
          </React.Fragment>
        </Paper>
      </Box>
    </PlainLayout>
  );
}
