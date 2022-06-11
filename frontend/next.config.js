const withTM = require('next-transpile-modules')([
  '@react-spring/three',
  '@react-spring/web',
]);

module.exports = withTM({
  // disable webpack 5 to make it compatible with the following rules
  webpack5: false,
  webpack: (config, options) => {
    config.module.rules.push({
      test: /react-spring/,
      sideEffects: true,
    });
    return config;
  },
});
