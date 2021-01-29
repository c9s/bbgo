const withTM = require('next-transpile-modules')
  ([
    '@react-spring/three',
    '@react-spring/web',
  ])

module.exports = withTM({
  webpack: config => {
    config.module.rules.push({
      test: /react-spring/,
      sideEffects: true,
    })

    return config
  },
})
