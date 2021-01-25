import axios from "axios";

const baseURL = process.env.NODE_ENV === "development" ? "http://localhost:8080" : ""

export function queryAssets(cb) {
    axios.get(baseURL + '/api/assets', {})
        .then(response => {
            cb(response.data.assets)
        });
}

