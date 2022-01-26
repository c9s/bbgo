pragma solidity 0.6.6;


contract Initializable {
    bool public inited = false;

    modifier initializer() {
        require(!inited, "already inited");
        _;
        inited = true;
    }
}
