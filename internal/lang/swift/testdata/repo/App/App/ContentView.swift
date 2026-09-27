import SwiftUI
import Alamofire
import Kingfisher
import CoreKit

struct ContentView: View {
    @StateObject var model = AppModel()
    var body: some View {
        Text(CoreThing.name)
    }
}
