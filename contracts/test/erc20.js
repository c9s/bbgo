const ChildMintableERC20 = artifacts.require("ChildMintableERC20");
contract("ChildMintableERC20", (accounts) => {
  it("should have BBG deployed", async () => {
    const instance = await ChildMintableERC20.deployed();
    const name = await instance.name();
    const decimal = await instance.decimals();
    const symbol = await instance.symbol();
    const balance = await instance.balanceOf(accounts[0]);
    const totalSupply = await instance.totalSupply();
    assert.equal(name.valueOf(), "BBGO");
    assert.equal(symbol.valueOf(), "BBG");
    assert.equal(decimal.toNumber(), 18);
    assert.equal(balance.toNumber(), 0);
    assert.equal(totalSupply.toNumber(), 0);
  });
});
