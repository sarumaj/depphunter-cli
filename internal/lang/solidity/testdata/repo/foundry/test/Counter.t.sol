// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test} from "forge-std/Test.sol";
import {Helper} from "util/Helper.sol";
import {Counter} from "../src/Counter.sol";
import {Mock} from "solmate/test/Mock.sol";

contract CounterTest is Test {
    Counter counter;

    function setUp() public {
        counter = new Counter();
    }

    function test_Increment() public {
        counter.increment();
    }
}
