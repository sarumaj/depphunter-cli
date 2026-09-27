// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

enum Rounding {
    Down,
    Up
}

uint256 constant WAD = 1e18;

type Price is uint128;

error MathOverflow();

event Computed(uint256 value);

struct Pair {
    uint256 a;
    uint256 b;
}

function mulWad(uint256 x, uint256 y) pure returns (uint256) {
    return (x * y) / WAD;
}

library Math {
    struct Frac {
        uint256 n;
        uint256 d;
    }

    enum Kind {
        A
    }

    uint256 internal constant ONE = 1;

    function max(uint256 a, uint256 b) internal pure returns (uint256) {
        return a > b ? a : b;
    }
}

using Math for uint256 global;
