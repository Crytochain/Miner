pragma solidity ^0.5.0;

library SafeMath {
    function add(uint256 a, uint256 b) internal pure returns (uint256) {
        uint256 res = a + b;
        assert(res >= a && res >= b);
        return res;
    }

    function sub(uint256 a, uint256 b) internal pure returns (uint256) {
        assert(a >= b);
        return a - b;
    }

    function mul(uint256 a, uint256 b) internal pure returns (uint256) {
        if(a == 0 || b == 0){
            return 0;
        }
        uint256 res = a * b;
        assert(a == res / b);
        return res;
    }

    function div(uint256 a, uint256 b) internal pure returns (uint256) {
        return a / b;
    }
}