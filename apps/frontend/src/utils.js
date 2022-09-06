export function currencyColor(currency) {
  switch (currency) {
    case 'BTC':
      return '#f69c3d';
    case 'ETH':
      return '#497493';
    case 'MCO':
      return '#032144';
    case 'OMG':
      return '#2159ec';
    case 'LTC':
      return '#949494';
    case 'USDT':
      return '#2ea07b';
    case 'SAND':
      return '#2E9AD0';
    case 'XRP':
      return '#00AAE4';
    case 'BCH':
      return '#8DC351';
    case 'MAX':
      return '#2D4692';
    case 'TWD':
      return '#4A7DED';
  }
}

export function throttle(fn, delayMillis) {
  let permitted = true;
  return () => {
    if (permitted) {
      fn.apply(this, arguments);
      permitted = false;
      setTimeout(() => (permitted = true), delayMillis);
    }
  };
}
