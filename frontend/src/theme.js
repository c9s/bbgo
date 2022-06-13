import { createTheme } from '@mui/material/styles';
import { red } from '@mui/material/colors';

// Create a theme instance.
const theme = createTheme({
  palette: {
    primary: {
      main: '#eb9534',
      contrastText: '#ffffff',
    },
    secondary: {
      main: '#ccc0b1',
      contrastText: '#eb9534',
    },
    error: {
      main: red.A400,
    },
    background: {
      default: '#fff',
    },
  },
});

export default theme;
