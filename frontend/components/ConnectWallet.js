import React from 'react';

import { makeStyles } from '@mui/styles';

import Button from '@mui/material/Button';
import ClickAwayListener from '@mui/material/ClickAwayListener';
import Grow from '@mui/material/Grow';
import Paper from '@mui/material/Paper';
import Popper from '@mui/material/Popper';
import MenuItem from '@mui/material/MenuItem';
import MenuList from '@mui/material/MenuList';
import ListItemText from '@mui/material/ListItemText';
import PersonIcon from '@mui/icons-material/Person';

import { useEtherBalance, useTokenBalance, useEthers } from '@usedapp/core';
import { formatEther } from '@ethersproject/units';

const useStyles = makeStyles((theme) => ({
  buttons: {
    margin: theme.spacing(1),
    padding: theme.spacing(1),
  },
  profile: {
    margin: theme.spacing(1),
    padding: theme.spacing(1),
  },
}));

const BBG = '0x3Afe98235d680e8d7A52e1458a59D60f45F935C0';

export default function ConnectWallet() {
  const classes = useStyles();

  const { activateBrowserWallet, account } = useEthers();
  const etherBalance = useEtherBalance(account);
  const tokenBalance = useTokenBalance(BBG, account);

  const [open, setOpen] = React.useState(false);
  const anchorRef = React.useRef(null);

  const handleToggle = () => {
    setOpen((prevOpen) => !prevOpen);
  };

  const handleClose = (event) => {
    if (anchorRef.current && anchorRef.current.contains(event.target)) {
      return;
    }

    setOpen(false);
  };

  function handleListKeyDown(event) {
    if (event.key === 'Tab') {
      event.preventDefault();
      setOpen(false);
    } else if (event.key === 'Escape') {
      setOpen(false);
    }
  }

  // return focus to the button when we transitioned from !open -> open
  const prevOpen = React.useRef(open);
  React.useEffect(() => {
    if (prevOpen.current === true && open === false) {
      anchorRef.current.focus();
    }

    prevOpen.current = open;
  }, [open]);

  return (
    <>
      {account ? (
        <>
          <Button
            ref={anchorRef}
            id="composition-button"
            aria-controls={open ? 'composition-menu' : undefined}
            aria-expanded={open ? 'true' : undefined}
            aria-haspopup="true"
            onClick={handleToggle}
          >
            <PersonIcon />
            <ListItemText primary="Profile" />
          </Button>
          <Popper
            open={open}
            anchorEl={anchorRef.current}
            role={undefined}
            placement="bottom-start"
            transition
            disablePortal
          >
            {({ TransitionProps, placement }) => (
              <Grow
                {...TransitionProps}
                style={{
                  transformOrigin:
                    placement === 'bottom-start' ? 'left top' : 'left bottom',
                }}
              >
                <Paper>
                  <ClickAwayListener onClickAway={handleClose}>
                    <MenuList
                      autoFocusItem={open}
                      id="composition-menu"
                      aria-labelledby="composition-button"
                      onKeyDown={handleListKeyDown}
                    >
                      <MenuItem onClick={handleClose}>
                        {account && <p>Account: {account}</p>}
                      </MenuItem>
                      <MenuItem onClick={handleClose}>
                        {etherBalance && (
                          <a>ETH Balance: {formatEther(etherBalance)}</a>
                        )}
                      </MenuItem>
                      <MenuItem onClick={handleClose}>
                        {tokenBalance && (
                          <a>BBG Balance: {formatEther(tokenBalance)}</a>
                        )}
                      </MenuItem>
                    </MenuList>
                  </ClickAwayListener>
                </Paper>
              </Grow>
            )}
          </Popper>
        </>
      ) : (
        <div>
          <button
            onClick={() => activateBrowserWallet()}
            className={classes.buttons}
          >
            Connect Wallet
          </button>
        </div>
      )}
    </>
  );
}
