// workaround for lightweight-charts under Next's webpack build, same as
// apps/backtest-report/next.config.js.
const withTM = require('next-transpile-modules')(['lightweight-charts', 'fancy-canvas']);

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
};

module.exports = withTM(nextConfig);
