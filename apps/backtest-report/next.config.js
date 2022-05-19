
// workaround for react financial charts
// https://github.com/react-financial/react-financial-charts/issues/606

// workaround for lightweight chart
// https://stackoverflow.com/questions/65936222/next-js-syntaxerror-unexpected-token-export
// https://stackoverflow.com/questions/66244968/cannot-use-import-statement-outside-a-module-error-when-importing-react-hook-m
const withTM = require('next-transpile-modules')([
    'lightweight-charts',
    'fancy-canvas',
    // 'd3-array',
    // 'd3-format',
    // 'd3-time',
    // 'd3-time-format',
    // 'react-financial-charts',
    // '@react-financial-charts/annotations',
    // '@react-financial-charts/axes',
    // '@react-financial-charts/coordinates',
    // '@react-financial-charts/core',
    // '@react-financial-charts/indicators',
    // '@react-financial-charts/interactive',
    // '@react-financial-charts/scales',
    // '@react-financial-charts/series',
    // '@react-financial-charts/tooltip',
    // '@react-financial-charts/utils',
]);

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
}

module.exports = withTM(nextConfig);
