import axios from "axios";

let baseURL = process.env.NODE_ENV === "development" ? "http://localhost:8080" : ""
baseURL = "http://localhost:8080"
export function ping(cb) {
    return axios.get(baseURL + '/api/ping').then(response => {
        cb(response.data)
    });
}

export function querySyncStatus(cb) {
    return axios.get(baseURL + '/api/environment/syncing').then(response => {
        cb(response.data.syncing)
    });
}

export function testDatabaseConnection(params, cb) {
    return axios.post(baseURL + '/api/setup/test-db', params).then(response => {
        cb(response.data)
    });
}

export function configureDatabase(params, cb) {
    return axios.post(baseURL + '/api/setup/configure-db', params).then(response => {
        cb(response.data)
    });
}

export function saveConfig(cb) {
    return axios.post(baseURL + '/api/setup/save').then(response => {
        cb(response.data)
    });
}

export function setupRestart(cb) {
    return axios.post(baseURL + '/api/setup/restart').then(response => {
        cb(response.data)
    });
}

export function addSession(session, cb) {
    return axios.post(baseURL + '/api/sessions', session).then(response => {
        cb(response.data || [])
    });
}

export function attachStrategyOn(session, strategyID, strategy, cb) {
    return axios.post(baseURL + `/api/setup/strategy/single/${strategyID}/session/${session}`, strategy).then(response => {
        cb(response.data)
    });
}

export function testSessionConnection(session, cb) {
    return axios.post(baseURL + '/api/sessions/test', session).then(response => {
        cb(response.data)
    });
}

export function queryStrategies(cb) {
    return axios.get(baseURL + '/api/strategies/single').then(response => {
        cb(response.data.strategies || [])
    });
}


export function querySessions(cb) {
    return axios.get(baseURL + '/api/sessions', {})
        .then(response => {
            cb(response.data.sessions || [])
        });
}

export function querySessionSymbols(sessionName, cb) {
    return axios.get(baseURL + `/api/sessions/${ sessionName }/symbols`, {})
        .then(response => {
            cb(response.data.symbols || [])
        });
}

export function queryTrades(params, cb) {
    axios.get(baseURL + '/api/trades', {params: params})
        .then(response => {
            cb(response.data.trades || [])
        });
}

export function queryClosedOrders(params, cb) {
    axios.get(baseURL + '/api/orders/closed', {params: params})
        .then(response => {
            cb(response.data.orders || [])
        });
}

export function queryAssets(cb) {
    axios.get(baseURL + '/api/assets', {})
        .then(response => {
            cb(response.data.assets || [])
        });
}

export function queryTradingVolume(params, cb) {
    axios.get(baseURL + '/api/trading-volume', {params: params})
        .then(response => {
            cb(response.data.tradingVolumes || [])
        });
}


