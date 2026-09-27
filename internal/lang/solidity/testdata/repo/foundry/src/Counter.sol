// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;
pragma abicoder v2;

import "./utils/Math.sol";
import {Math as M, Rounding} from "./utils/Math.sol";
import * as Lib from "local/Local.sol";
import "util/Math.sol" as UtilMath;
import {Test} from "forge-std/Test.sol";
import {ERC20} from "@oz/token/ERC20/ERC20.sol";
import {ERC20 as SoldeerERC20} from "@openzeppelin-contracts-5.0.2/token/ERC20/ERC20.sol";
import {LibString} from "solady-0.0.200/src/utils/LibString.sol";
import {DSTest} from "ds-test/test.sol";
import {Vault} from "vault/Vault.sol";
import "gone/Nothing.sol";
import "../../outside.sol";
import {Unknown} from "unknown-lib/Unknown.sol";
import {Scoped} from "@scope/pkg/Scoped.sol";
import "src/local/Local.sol";

/// @notice import "natspec/Fake.sol";
/**
 * import "block/Fake.sol";
 */
contract Counter is Test {
    uint256 public constant MAX = 10;
    uint256 public number;
    string private note = 'import "string/Fake.sol";';

    event Incremented(uint256 by);
    error TooLarge(uint256 value);

    modifier small(uint256 x) {
        if (x > MAX) revert TooLarge(x);
        _;
    }

    constructor() {
        number = 0;
    }

    function increment(uint256 by) public small(by) {
        number += by;
        emit Incremented(by);
    }

    function increment() external {
        number++;
    }

    function hash(bytes memory data) internal pure returns (bytes32 h) {
        assembly ("memory-safe") {
            function inner(p) -> r { r := p }
            h := keccak256(add(data, 0x20), mload(data))
            let s := "import \"asm/Fake.sol\";"
        }
    }

    receive() external payable {}

    fallback() external {}
}
