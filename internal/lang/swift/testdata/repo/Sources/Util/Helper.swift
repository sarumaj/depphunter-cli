import Collections
import LinkedList

public struct Helper {
    public init() {}
}

extension Array where Element == Int {
    func sum() -> Int { reduce(0, +) }
}
