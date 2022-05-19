import {Market} from './market';

export interface ReportEntry {
  id: string;

  config: object;

  time: string;
}

export interface ReportIndex {
  runs: Array<ReportEntry>;
}

export interface Balance {
  currency: string;
  available: number;
  locked: number;
  borrowed: number;
}

export interface BalanceMap {
  [currency: string]: Balance;
}

export interface ReportSummary {
  startTime: Date;
  endTime: Date;
  sessions: string[];
  symbols: string[];
  intervals: string[];
  initialTotalBalances: BalanceMap;
  finalTotalBalances: BalanceMap;
  symbolReports: SymbolReport[];
  manifests: Manifest[];
}

export interface SymbolReport {
  exchange: string;
  symbol: string;
  market: Market;
  lastPrice: number;
  startPrice: number;
  pnl: PnL;
  initialBalances: BalanceMap;
  finalBalances: BalanceMap;
}


export interface Manifest {
  type: string;
  filename: string;
  strategyID: string;
  strategyInstance: string;
  strategyProperty: string;
}

export interface CurrencyFeeMap {
  [currency: string]: number;
}

export interface PnL {
  lastPrice: number;
  startTime: Date;
  symbol: string;
  market: Market;
  numTrades: number;
  profit: number;
  netProfit: number;
  unrealizedProfit: number;
  averageCost: number;
  buyVolume: number;
  sellVolume: number;
  feeInUSD: number;
  currencyFees: CurrencyFeeMap;
}
