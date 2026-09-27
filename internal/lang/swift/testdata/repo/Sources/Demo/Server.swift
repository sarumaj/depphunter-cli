import Foundation
import NIOCore
@preconcurrency import Logging
@_exported import Util
import struct Collections.Deque
import Markdown
#if canImport(UIKit)
import UIKit
#else
import AppKit
#endif

public final class Server: Service {
    let logger = Logger(label: "demo")
    private var queue: Deque<Job> = []
    static let shared = Server()

    public init(port: Int) {
        let helper = Helper()
        _ = helper
    }

    public func start() throws -> Status {
        func local() {}
        return .running
    }

    #if DEBUG
    func debugDump() {}
    #endif
}

extension Server: CustomStringConvertible {
    public var description: String { "server" }
}

func makeServer() -> Server { Server(port: 8080) }
let defaultPort = 8080
