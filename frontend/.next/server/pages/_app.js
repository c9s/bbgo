module.exports =
/******/ (function(modules) { // webpackBootstrap
/******/ 	// The module cache
/******/ 	var installedModules = require('../ssr-module-cache.js');
/******/
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/
/******/ 		// Check if module is in cache
/******/ 		if(installedModules[moduleId]) {
/******/ 			return installedModules[moduleId].exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = installedModules[moduleId] = {
/******/ 			i: moduleId,
/******/ 			l: false,
/******/ 			exports: {}
/******/ 		};
/******/
/******/ 		// Execute the module function
/******/ 		var threw = true;
/******/ 		try {
/******/ 			modules[moduleId].call(module.exports, module, module.exports, __webpack_require__);
/******/ 			threw = false;
/******/ 		} finally {
/******/ 			if(threw) delete installedModules[moduleId];
/******/ 		}
/******/
/******/ 		// Flag the module as loaded
/******/ 		module.l = true;
/******/
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/
/******/
/******/ 	// expose the modules object (__webpack_modules__)
/******/ 	__webpack_require__.m = modules;
/******/
/******/ 	// expose the module cache
/******/ 	__webpack_require__.c = installedModules;
/******/
/******/ 	// define getter function for harmony exports
/******/ 	__webpack_require__.d = function(exports, name, getter) {
/******/ 		if(!__webpack_require__.o(exports, name)) {
/******/ 			Object.defineProperty(exports, name, { enumerable: true, get: getter });
/******/ 		}
/******/ 	};
/******/
/******/ 	// define __esModule on exports
/******/ 	__webpack_require__.r = function(exports) {
/******/ 		if(typeof Symbol !== 'undefined' && Symbol.toStringTag) {
/******/ 			Object.defineProperty(exports, Symbol.toStringTag, { value: 'Module' });
/******/ 		}
/******/ 		Object.defineProperty(exports, '__esModule', { value: true });
/******/ 	};
/******/
/******/ 	// create a fake namespace object
/******/ 	// mode & 1: value is a module id, require it
/******/ 	// mode & 2: merge all properties of value into the ns
/******/ 	// mode & 4: return value when already ns object
/******/ 	// mode & 8|1: behave like require
/******/ 	__webpack_require__.t = function(value, mode) {
/******/ 		if(mode & 1) value = __webpack_require__(value);
/******/ 		if(mode & 8) return value;
/******/ 		if((mode & 4) && typeof value === 'object' && value && value.__esModule) return value;
/******/ 		var ns = Object.create(null);
/******/ 		__webpack_require__.r(ns);
/******/ 		Object.defineProperty(ns, 'default', { enumerable: true, value: value });
/******/ 		if(mode & 2 && typeof value != 'string') for(var key in value) __webpack_require__.d(ns, key, function(key) { return value[key]; }.bind(null, key));
/******/ 		return ns;
/******/ 	};
/******/
/******/ 	// getDefaultExport function for compatibility with non-harmony modules
/******/ 	__webpack_require__.n = function(module) {
/******/ 		var getter = module && module.__esModule ?
/******/ 			function getDefault() { return module['default']; } :
/******/ 			function getModuleExports() { return module; };
/******/ 		__webpack_require__.d(getter, 'a', getter);
/******/ 		return getter;
/******/ 	};
/******/
/******/ 	// Object.prototype.hasOwnProperty.call
/******/ 	__webpack_require__.o = function(object, property) { return Object.prototype.hasOwnProperty.call(object, property); };
/******/
/******/ 	// __webpack_public_path__
/******/ 	__webpack_require__.p = "/_next/";
/******/
/******/
/******/ 	// Load entry module and return exports
/******/ 	return __webpack_require__(__webpack_require__.s = 0);
/******/ })
/************************************************************************/
/******/ ({

/***/ "+FwM":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/colors");

/***/ }),

/***/ 0:
/***/ (function(module, exports, __webpack_require__) {

module.exports = __webpack_require__("cha2");


/***/ }),

/***/ "0Jp5":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/DialogTitle");

/***/ }),

/***/ "0zb8":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/LinearProgress");

/***/ }),

/***/ "9Pu4":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/styles");

/***/ }),

/***/ "AJJM":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/CssBaseline");

/***/ }),

/***/ "F5FC":
/***/ (function(module, exports) {

module.exports = require("react/jsx-runtime");

/***/ }),

/***/ "FVNg":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return ping; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "j", function() { return querySyncStatus; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "o", function() { return testDatabaseConnection; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return configureDatabase; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "m", function() { return saveConfig; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "n", function() { return setupRestart; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return addSession; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return attachStrategyOn; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "p", function() { return testSessionConnection; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "i", function() { return queryStrategies; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "h", function() { return querySessions; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "g", function() { return querySessionSymbols; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "k", function() { return queryTrades; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "f", function() { return queryClosedOrders; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return queryAssets; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "l", function() { return queryTradingVolume; });
/* harmony import */ var axios__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("zr5I");
/* harmony import */ var axios__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(axios__WEBPACK_IMPORTED_MODULE_0__);

let baseURL = false ? undefined : "";
baseURL = "http://66.228.52.222:8080";
function ping(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/ping').then(response => {
    cb(response.data);
  });
}
function querySyncStatus(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/environment/syncing').then(response => {
    cb(response.data.syncing);
  });
}
function testDatabaseConnection(params, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/setup/test-db', params).then(response => {
    cb(response.data);
  });
}
function configureDatabase(params, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/setup/configure-db', params).then(response => {
    cb(response.data);
  });
}
function saveConfig(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/setup/save').then(response => {
    cb(response.data);
  });
}
function setupRestart(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/setup/restart').then(response => {
    cb(response.data);
  });
}
function addSession(session, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/sessions', session).then(response => {
    cb(response.data || []);
  });
}
function attachStrategyOn(session, strategyID, strategy, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + `/api/setup/strategy/single/${strategyID}/session/${session}`, strategy).then(response => {
    cb(response.data);
  });
}
function testSessionConnection(session, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.post(baseURL + '/api/sessions/test', session).then(response => {
    cb(response.data);
  });
}
function queryStrategies(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/strategies/single').then(response => {
    cb(response.data.strategies || []);
  });
}
function querySessions(cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/sessions', {}).then(response => {
    cb(response.data.sessions || []);
  });
}
function querySessionSymbols(sessionName, cb) {
  return axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + `/api/sessions/${sessionName}/symbols`, {}).then(response => {
    cb(response.data.symbols || []);
  });
}
function queryTrades(params, cb) {
  axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/trades', {
    params: params
  }).then(response => {
    cb(response.data.trades || []);
  });
}
function queryClosedOrders(params, cb) {
  axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/orders/closed', {
    params: params
  }).then(response => {
    cb(response.data.orders || []);
  });
}
function queryAssets(cb) {
  axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/assets', {}).then(response => {
    cb(response.data.assets || []);
  });
}
function queryTradingVolume(params, cb) {
  axios__WEBPACK_IMPORTED_MODULE_0___default.a.get(baseURL + '/api/trading-volume', {
    params: params
  }).then(response => {
    cb(response.data.tradingVolumes || []);
  });
}

/***/ }),

/***/ "MbIc":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/DialogContentText");

/***/ }),

/***/ "ZkBw":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/Box");

/***/ }),

/***/ "cDcd":
/***/ (function(module, exports) {

module.exports = require("react");

/***/ }),

/***/ "cha2":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
__webpack_require__.r(__webpack_exports__);
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "default", function() { return MyApp; });
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("cDcd");
/* harmony import */ var react__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(react__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var next_head__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("xnum");
/* harmony import */ var next_head__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(next_head__WEBPACK_IMPORTED_MODULE_1__);
/* harmony import */ var _material_ui_core_styles__WEBPACK_IMPORTED_MODULE_2__ = __webpack_require__("9Pu4");
/* harmony import */ var _material_ui_core_styles__WEBPACK_IMPORTED_MODULE_2___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_styles__WEBPACK_IMPORTED_MODULE_2__);
/* harmony import */ var _material_ui_core_Dialog__WEBPACK_IMPORTED_MODULE_3__ = __webpack_require__("fEgT");
/* harmony import */ var _material_ui_core_Dialog__WEBPACK_IMPORTED_MODULE_3___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_Dialog__WEBPACK_IMPORTED_MODULE_3__);
/* harmony import */ var _material_ui_core_DialogContent__WEBPACK_IMPORTED_MODULE_4__ = __webpack_require__("iTUb");
/* harmony import */ var _material_ui_core_DialogContent__WEBPACK_IMPORTED_MODULE_4___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_DialogContent__WEBPACK_IMPORTED_MODULE_4__);
/* harmony import */ var _material_ui_core_DialogContentText__WEBPACK_IMPORTED_MODULE_5__ = __webpack_require__("MbIc");
/* harmony import */ var _material_ui_core_DialogContentText__WEBPACK_IMPORTED_MODULE_5___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_DialogContentText__WEBPACK_IMPORTED_MODULE_5__);
/* harmony import */ var _material_ui_core_DialogTitle__WEBPACK_IMPORTED_MODULE_6__ = __webpack_require__("0Jp5");
/* harmony import */ var _material_ui_core_DialogTitle__WEBPACK_IMPORTED_MODULE_6___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_DialogTitle__WEBPACK_IMPORTED_MODULE_6__);
/* harmony import */ var _material_ui_core_LinearProgress__WEBPACK_IMPORTED_MODULE_7__ = __webpack_require__("0zb8");
/* harmony import */ var _material_ui_core_LinearProgress__WEBPACK_IMPORTED_MODULE_7___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_LinearProgress__WEBPACK_IMPORTED_MODULE_7__);
/* harmony import */ var _material_ui_core_Box__WEBPACK_IMPORTED_MODULE_8__ = __webpack_require__("ZkBw");
/* harmony import */ var _material_ui_core_Box__WEBPACK_IMPORTED_MODULE_8___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_Box__WEBPACK_IMPORTED_MODULE_8__);
/* harmony import */ var _material_ui_core_CssBaseline__WEBPACK_IMPORTED_MODULE_9__ = __webpack_require__("AJJM");
/* harmony import */ var _material_ui_core_CssBaseline__WEBPACK_IMPORTED_MODULE_9___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_CssBaseline__WEBPACK_IMPORTED_MODULE_9__);
/* harmony import */ var _src_theme__WEBPACK_IMPORTED_MODULE_10__ = __webpack_require__("zDcZ");
/* harmony import */ var _styles_globals_css__WEBPACK_IMPORTED_MODULE_11__ = __webpack_require__("zPlV");
/* harmony import */ var _styles_globals_css__WEBPACK_IMPORTED_MODULE_11___default = /*#__PURE__*/__webpack_require__.n(_styles_globals_css__WEBPACK_IMPORTED_MODULE_11__);
/* harmony import */ var _api_bbgo__WEBPACK_IMPORTED_MODULE_12__ = __webpack_require__("FVNg");
/* harmony import */ var react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__ = __webpack_require__("F5FC");
/* harmony import */ var react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13___default = /*#__PURE__*/__webpack_require__.n(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__);
function ownKeys(object, enumerableOnly) { var keys = Object.keys(object); if (Object.getOwnPropertySymbols) { var symbols = Object.getOwnPropertySymbols(object); if (enumerableOnly) { symbols = symbols.filter(function (sym) { return Object.getOwnPropertyDescriptor(object, sym).enumerable; }); } keys.push.apply(keys, symbols); } return keys; }

function _objectSpread(target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i] != null ? arguments[i] : {}; if (i % 2) { ownKeys(Object(source), true).forEach(function (key) { _defineProperty(target, key, source[key]); }); } else if (Object.getOwnPropertyDescriptors) { Object.defineProperties(target, Object.getOwnPropertyDescriptors(source)); } else { ownKeys(Object(source)).forEach(function (key) { Object.defineProperty(target, key, Object.getOwnPropertyDescriptor(source, key)); }); } } return target; }

function _defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }
















const SyncNotStarted = 0;
const Syncing = 1;
const SyncDone = 2; // session is configured, check if we're syncing data

let syncStatusPoller = null;
function MyApp(props) {
  const {
    Component,
    pageProps
  } = props;
  const [loading, setLoading] = react__WEBPACK_IMPORTED_MODULE_0___default.a.useState(true);
  const [syncing, setSyncing] = react__WEBPACK_IMPORTED_MODULE_0___default.a.useState(false);
  react__WEBPACK_IMPORTED_MODULE_0___default.a.useEffect(() => {
    // Remove the server-side injected CSS.
    const jssStyles = document.querySelector('#jss-server-side');

    if (jssStyles) {
      jssStyles.parentElement.removeChild(jssStyles);
    }

    Object(_api_bbgo__WEBPACK_IMPORTED_MODULE_12__[/* querySessions */ "h"])(sessions => {
      if (sessions.length > 0) {
        setSyncing(true);

        const pollSyncStatus = () => {
          Object(_api_bbgo__WEBPACK_IMPORTED_MODULE_12__[/* querySyncStatus */ "j"])(status => {
            switch (status) {
              case SyncNotStarted:
                break;

              case Syncing:
                setSyncing(true);
                break;

              case SyncDone:
                clearInterval(syncStatusPoller);
                setLoading(false);
                setSyncing(false);
                break;
            }
          }).catch(err => {
            console.error(err);
          });
        };

        syncStatusPoller = setInterval(pollSyncStatus, 1000);
      } else {
        // no session found, so we can not sync any data
        setLoading(false);
        setSyncing(false);
      }
    }).catch(err => {
      console.error(err);
    });
  }, []);
  return /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(react__WEBPACK_IMPORTED_MODULE_0___default.a.Fragment, {
    children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(next_head__WEBPACK_IMPORTED_MODULE_1___default.a, {
      children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])("title", {
        children: "BBGO"
      }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])("meta", {
        name: "viewport",
        content: "minimum-scale=1, initial-scale=1, width=device-width"
      })]
    }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(_material_ui_core_styles__WEBPACK_IMPORTED_MODULE_2__["ThemeProvider"], {
      theme: _src_theme__WEBPACK_IMPORTED_MODULE_10__[/* default */ "a"],
      children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_CssBaseline__WEBPACK_IMPORTED_MODULE_9___default.a, {}), loading ? syncing ? /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(react__WEBPACK_IMPORTED_MODULE_0___default.a.Fragment, {
        children: /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(_material_ui_core_Dialog__WEBPACK_IMPORTED_MODULE_3___default.a, {
          open: syncing,
          "aria-labelledby": "alert-dialog-title",
          "aria-describedby": "alert-dialog-description",
          children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_DialogTitle__WEBPACK_IMPORTED_MODULE_6___default.a, {
            id: "alert-dialog-title",
            children: "Syncing Trades"
          }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(_material_ui_core_DialogContent__WEBPACK_IMPORTED_MODULE_4___default.a, {
            children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_DialogContentText__WEBPACK_IMPORTED_MODULE_5___default.a, {
              id: "alert-dialog-description",
              children: "The environment is syncing trades from the exchange sessions. Please wait a moment..."
            }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_Box__WEBPACK_IMPORTED_MODULE_8___default.a, {
              m: 2,
              children: /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_LinearProgress__WEBPACK_IMPORTED_MODULE_7___default.a, {})
            })]
          })]
        })
      }) : /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(react__WEBPACK_IMPORTED_MODULE_0___default.a.Fragment, {
        children: /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(_material_ui_core_Dialog__WEBPACK_IMPORTED_MODULE_3___default.a, {
          open: loading,
          "aria-labelledby": "alert-dialog-title",
          "aria-describedby": "alert-dialog-description",
          children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_DialogTitle__WEBPACK_IMPORTED_MODULE_6___default.a, {
            id: "alert-dialog-title",
            children: "Loading"
          }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsxs"])(_material_ui_core_DialogContent__WEBPACK_IMPORTED_MODULE_4___default.a, {
            children: [/*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_DialogContentText__WEBPACK_IMPORTED_MODULE_5___default.a, {
              id: "alert-dialog-description",
              children: "Loading..."
            }), /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_Box__WEBPACK_IMPORTED_MODULE_8___default.a, {
              m: 2,
              children: /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(_material_ui_core_LinearProgress__WEBPACK_IMPORTED_MODULE_7___default.a, {})
            })]
          })]
        })
      }) : /*#__PURE__*/Object(react_jsx_runtime__WEBPACK_IMPORTED_MODULE_13__["jsx"])(Component, _objectSpread({}, pageProps))]
    })]
  });
}

/***/ }),

/***/ "fEgT":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/Dialog");

/***/ }),

/***/ "iTUb":
/***/ (function(module, exports) {

module.exports = require("@material-ui/core/DialogContent");

/***/ }),

/***/ "xnum":
/***/ (function(module, exports) {

module.exports = require("next/head");

/***/ }),

/***/ "zDcZ":
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var _material_ui_core_styles__WEBPACK_IMPORTED_MODULE_0__ = __webpack_require__("9Pu4");
/* harmony import */ var _material_ui_core_styles__WEBPACK_IMPORTED_MODULE_0___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_styles__WEBPACK_IMPORTED_MODULE_0__);
/* harmony import */ var _material_ui_core_colors__WEBPACK_IMPORTED_MODULE_1__ = __webpack_require__("+FwM");
/* harmony import */ var _material_ui_core_colors__WEBPACK_IMPORTED_MODULE_1___default = /*#__PURE__*/__webpack_require__.n(_material_ui_core_colors__WEBPACK_IMPORTED_MODULE_1__);

 // Create a theme instance.

const theme = Object(_material_ui_core_styles__WEBPACK_IMPORTED_MODULE_0__["createTheme"])({
  palette: {
    primary: {
      main: '#eb9534',
      contrastText: '#ffffff'
    },
    secondary: {
      main: '#ccc0b1',
      contrastText: '#eb9534'
    },
    error: {
      main: _material_ui_core_colors__WEBPACK_IMPORTED_MODULE_1__["red"].A400
    },
    background: {
      default: '#fff'
    }
  }
});
/* harmony default export */ __webpack_exports__["a"] = (theme);

/***/ }),

/***/ "zPlV":
/***/ (function(module, exports) {



/***/ }),

/***/ "zr5I":
/***/ (function(module, exports) {

module.exports = require("axios");

/***/ })

/******/ });