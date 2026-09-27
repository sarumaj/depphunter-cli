import XCTest
@testable import Demo

final class ServerTests: XCTestCase {
    func testStart() throws {
        XCTAssertEqual(try Server.shared.start(), Status.running)
    }
}
