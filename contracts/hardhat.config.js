const { etherscanApiKey } = require('./secrets.json');
require("@nomiclabs/hardhat-etherscan");

module.exports = {
  solidity: "0.6.12",
  networks: {
  },
  etherscan: {
    apiKey: etherscanApiKey
  }
};
