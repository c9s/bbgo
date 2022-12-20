// Linregmaker is a maker strategy depends on the linear regression baseline slopes
//
// Linregmaker uses two linear regression baseline slopes for trading:
// 1) The fast linReg is to determine the short-term trend. It controls whether placing buy/sell orders or not.
// 2) The slow linReg is to determine the mid-term trend. It controls whether the creation of opposite direction position is allowed.
package linregmaker
