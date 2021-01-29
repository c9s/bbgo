import Drawer from "@material-ui/core/Drawer";
import Divider from "@material-ui/core/Divider";
import List from "@material-ui/core/List";
import Link from "next/link";
import ListItem from "@material-ui/core/ListItem";
import ListItemIcon from "@material-ui/core/ListItemIcon";
import DashboardIcon from "@material-ui/icons/Dashboard";
import ListItemText from "@material-ui/core/ListItemText";
import ListIcon from "@material-ui/icons/List";
import TrendingUpIcon from "@material-ui/icons/TrendingUp";
import React from "react";
import {makeStyles} from "@material-ui/core/styles";

const drawerWidth = 260;

const useStyles = makeStyles((theme) => ({
    root: {
        // flexGrow: 1,
        display: 'flex',
    },
    content: {
        flexGrow: 1,
        height: '100vh',
        overflow: 'auto',
    },
    fixedHeight: {
        height: 240,
    },
    toolbar: {
        paddingRight: 24, // keep right padding when drawer closed
    },
    toolbarIcon: {
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        padding: '0 8px',
        ...theme.mixins.toolbar,
    },
    appBarSpacer: theme.mixins.toolbar,
    drawerPaper: {
        position: 'relative',
        whiteSpace: 'nowrap',
        width: drawerWidth,
        transition: theme.transitions.create('width', {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.enteringScreen,
        }),
    },
}));


export default function SideBar() {
    const classes = useStyles();

    return <Drawer
        variant="permanent"
        className={classes.drawerPaper}
        open={true}>

        <div className={classes.appBarSpacer}/>

        <List>
            <Link href={"/"}>
                <ListItem button>
                    <ListItemIcon>
                        <DashboardIcon/>
                    </ListItemIcon>
                    <ListItemText primary="Dashboard"/>
                </ListItem>
            </Link>
        </List>
        <Divider/>
        <List>
            <Link href={"/orders"}>
                <ListItem button>
                    <ListItemIcon>
                        <ListIcon/>
                    </ListItemIcon>
                    <ListItemText primary="Orders"/>
                </ListItem>
            </Link>
            <Link href={"/trades"}>
                <ListItem button>
                    <ListItemIcon>
                        <ListIcon/>
                    </ListItemIcon>
                    <ListItemText primary="Trades"/>
                </ListItem>
            </Link>
            <ListItem button>
                <ListItemIcon>
                    <TrendingUpIcon/>
                </ListItemIcon>
                <ListItemText primary="Strategies"/>
            </ListItem>
        </List>
    </Drawer>


}
