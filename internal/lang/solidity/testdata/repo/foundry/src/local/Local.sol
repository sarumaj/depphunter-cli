// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ILocal {
    function get() external view returns (uint256);
    event Got(uint256 v);
}
