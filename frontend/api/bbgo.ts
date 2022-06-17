import axios from 'axios';

const baseURL =
  process.env.NODE_ENV === 'development' ? 'http://localhost:8080' : '';

export function ping(cb) {
  return axios.get(baseURL + '/api/ping').then((response) => {
    cb(response.data);
  });
}

export function queryOutboundIP(cb) {
  return axios.get<any>(baseURL + '/api/outbound-ip').then((response) => {
    cb(response.data.outboundIP);
  });
}

export async function triggerSync() {
  return axios.post<any>(baseURL + '/api/environment/sync');
}

export enum SyncStatus {
  SyncNotStarted = 0,
  Syncing = 1,
  SyncDone = 2,
}

export async function querySyncStatus(): Promise<SyncStatus> {
  const resp = await axios.get<any>(baseURL + '/api/environment/syncing');
  return resp.data.syncing;
}

export function testDatabaseConnection(params, cb) {
  return axios.post(baseURL + '/api/setup/test-db', params).then((response) => {
    cb(response.data);
  });
}

export function configureDatabase(params, cb) {
  return axios
    .post(baseURL + '/api/setup/configure-db', params)
    .then((response) => {
      cb(response.data);
    });
}

export function saveConfig(cb) {
  return axios.post(baseURL + '/api/setup/save').then((response) => {
    cb(response.data);
  });
}

export function setupRestart(cb) {
  return axios.post(baseURL + '/api/setup/restart').then((response) => {
    cb(response.data);
  });
}

export function addSession(session, cb) {
  return axios.post(baseURL + '/api/sessions', session).then((response) => {
    cb(response.data || []);
  });
}

export function attachStrategyOn(session, strategyID, strategy, cb) {
  return axios
    .post(
      baseURL + `/api/setup/strategy/single/${strategyID}/session/${session}`,
      strategy
    )
    .then((response) => {
      cb(response.data);
    });
}

export function testSessionConnection(session, cb) {
  return axios
    .post(baseURL + '/api/sessions/test', session)
    .then((response) => {
      cb(response.data);
    });
}

export function queryStrategies(cb) {
  return axios.get<any>(baseURL + '/api/strategies/single').then((response) => {
    cb(response.data.strategies || []);
  });
}

export function querySessions(cb) {
  return axios.get<any>(baseURL + '/api/sessions', {}).then((response) => {
    cb(response.data.sessions || []);
  });
}

export function querySessionSymbols(sessionName, cb) {
  return axios
    .get<any>(baseURL + `/api/sessions/${sessionName}/symbols`, {})
    .then((response) => {
      cb(response.data?.symbols || []);
    });
}

export function queryTrades(params, cb) {
  axios
    .get<any>(baseURL + '/api/trades', { params: params })
    .then((response) => {
      cb(response.data.trades || []);
    });
}

export function queryClosedOrders(params, cb) {
  axios
    .get<any>(baseURL + '/api/orders/closed', { params: params })
    .then((response) => {
      cb(response.data.orders || []);
    });
}

export function queryAssets(cb) {
  axios.get<any>(baseURL + '/api/assets', {}).then((response) => {
    cb(response.data.assets || []);
  });
}

export function queryTradingVolume(params, cb) {
  axios
    .get<any>(baseURL + '/api/trading-volume', { params: params })
    .then((response) => {
      cb(response.data.tradingVolumes || []);
    });
}

export interface GridStrategy {
  id: string;
  instanceID: string;
  strategy: string;
  grid: {
    symbol: string;
  };
  stats: GridStats;
  status: string;
  startTime: number;
}

export interface GridStats {
  oneDayArbs: number;
  totalArbs: number;
  investment: number;
  totalProfits: number;
  gridProfits: number;
  floatingPNL: number;
  currentPrice: number;
  lowestPrice: number;
  highestPrice: number;
}

export async function queryStrategiesMetrics(): Promise<GridStrategy[]> {
  const temp = {
    id: 'uuid',
    instanceID: 'testInstanceID',
    strategy: 'grid',
    grid: {
      symbol: 'BTCUSDT',
    },
    stats: {
      oneDayArbs: 0,
      totalArbs: 3,
      investment: 100,
      totalProfits: 5.6,
      gridProfits: 2.5,
      floatingPNL: 3.1,
      currentPrice: 29000,
      lowestPrice: 25000,
      highestPrice: 35000,
    },
    status: 'RUNNING',
    startTime: 1654938187102,
  };

  const testArr = [];

  for (let i = 0; i < 11; i++) {
    const cloned = { ...temp };
    cloned.id = 'uuid' + i;
    testArr.push(cloned);
  }

  return testArr;
}
