import React from 'react';

import { makeStyles } from '@material-ui/core/styles';

import Button from '@material-ui/core/Button';
import ClickAwayListener from '@material-ui/core/ClickAwayListener';
import Grow from '@material-ui/core/Grow';
import Paper from '@material-ui/core/Paper';
import Popper from '@material-ui/core/Popper';
import MenuItem from '@material-ui/core/MenuItem';
import MenuList from '@material-ui/core/MenuList';
import ListItemText from '@material-ui/core/ListItemText';
import PersonIcon from '@material-ui/icons/Person';

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
