import axios from "axios";

const baseURL = process.env.NODE_ENV === "development" ? "http://localhost:8080" : ""

export function testDatabaseConnection(dsn, cb) {
    return axios.post(baseURL + '/api/setup/test-db', {dsn: dsn}).then(response => {
        cb(response.data)
    });
}

export function configureDatabase(dsn, cb) {
    return axios.post(baseURL + '/api/setup/configure-db', {dsn: dsn}).then(response => {
        cb(response.data)
    });
}

export function addSession(session, cb) {
    return axios.post(baseURL + '/api/sessions', session).then(response => {
        cb(response.data)
    });
}

export function testSessionConnection(session, cb) {
    return axios.post(baseURL + '/api/sessions/test', session).then(response => {
        cb(response.data)
    });
}

export function querySessions(cb) {
    axios.get(baseURL + '/api/sessions', {})
        .then(response => {
            cb(response.data.sessions)
        });
}

export function queryTrades(params, cb) {
    axios.get(baseURL + '/api/trades', {params: params})
        .then(response => {
            cb(response.data.trades)
        });
}

export function queryClosedOrders(params, cb) {
    axios.get(baseURL + '/api/orders/closed', {params: params})
        .then(response => {
            cb(response.data.orders)
        });
}

export function queryAssets(cb) {
    axios.get(baseURL + '/api/assets', {})
        .then(response => {
            cb(response.data.assets)
        });
}

export function queryTradingVolume(params, cb) {
    axios.get(baseURL + '/api/trading-volume', {params: params})
        .then(response => {
            cb(response.data.tradingVolumes)
        });
}


