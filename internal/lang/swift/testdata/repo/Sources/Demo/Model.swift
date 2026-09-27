public protocol Service {
    func start() throws -> Status
}

public enum Status: String {
    case running, stopped
    func label() -> String { rawValue }
}

struct Job {
    struct Step {}
}

private struct Hidden {}

public actor Registry {}

public typealias Handler = (Job) -> Void
