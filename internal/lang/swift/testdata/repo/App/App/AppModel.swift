import Combine

@MainActor
final class AppModel: ObservableObject {
    @Published var count = 0
}
