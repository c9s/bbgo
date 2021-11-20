var ChildMintableERC20 = artifacts.require("./ChildMintableERC20.sol");

module.exports = function(deployer) {
  deployer.deploy(ChildMintableERC20, 'BBGO', 'BBG', 18, '0xA6FA4fB5f76172d178d61B04b0ecd319C5d1C0aa');
};
