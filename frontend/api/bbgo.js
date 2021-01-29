import axios from "axios";

const baseURL = process.env.NODE_ENV === "development" ? "http://localhost:8080" : ""

export function querySessions(cb) {
    axios.get(baseURL + '/api/sessions', {})
        .then(response => {
            cb(response.data.sessions)
        });
}

export function queryOrders(params, cb) {
    axios.get(baseURL + '/api/orders', { params: params })
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
    axios.get(baseURL + '/api/trading-volume', { params: params })
        .then(response => {
            cb(response.data.tradingVolumes)
        });
}


