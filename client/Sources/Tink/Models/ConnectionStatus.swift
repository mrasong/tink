import Foundation

public enum ConnectionStatus: Equatable {
    case disconnected
    case connecting
    case connected
    case error(String)

    public var title: String {
        switch self {
        case .disconnected:
            return String(localized: "Disconnected")
        case .connecting:
            return String(localized: "Connecting...")
        case .connected:
            return String(localized: "Connected")
        case .error(let msg):
            return String(format: String(localized: "Error: %1$@"), msg)
        }
    }

    public var symbol: String {
        switch self {
        case .disconnected:
            return "circle"
        case .connecting:
            return "circle.lefthalf.filled"
        case .connected:
            return "circle.fill"
        case .error:
            return "exclamationmark.circle.fill"
        }
    }
}
