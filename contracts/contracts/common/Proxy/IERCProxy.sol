pragma solidity 0.6.6;

interface IERCProxy {
    function proxyType() external pure returns (uint256 proxyTypeId);

    function implementation() external view returns (address codeAddr);
}
