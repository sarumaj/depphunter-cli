// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "hardhat/console.sol";
import {LibString} from "solady/src/utils/LibString.sol";
import {Lib} from "./lib/Lib.sol";
import "contracts/lib/Lib.sol";
import {Missing} from "@unknown/pkg/Missing.sol";
import 'openzeppelin-solidity/contracts/Old.sol';

contract Token is ERC20 {
    constructor() ERC20(unicode"Tøken", hex"00") {}
}
