import Foundation

public enum NotificationGroup: Hashable {
    case all
    case unread
    case starred
    case general
    case custom(String)

    public init(rawValue: String) {
        switch rawValue {
        case "__all__": self = .all
        case "__unread__": self = .unread
        case "__starred__": self = .starred
        case "__general__": self = .general
        default: self = .custom(rawValue)
        }
    }

    public var rawValue: String {
        switch self {
        case .all: return "__all__"
        case .unread: return "__unread__"
        case .starred: return "__starred__"
        case .general: return "__general__"
        case .custom(let name): return name
        }
    }

    public var title: String {
        switch self {
        case .all: return String(localized: "All Notifications")
        case .unread: return String(localized: "Unread")
        case .starred: return String(localized: "Starred")
        case .general: return String(localized: "General")
        case .custom(let name): return "# \(name)"
        }
    }

    public var icon: String {
        switch self {
        case .all: return "tray.fill"
        case .unread: return "circle.fill"
        case .starred: return "star.fill"
        case .general: return "bell.fill"
        case .custom: return "tag.fill"
        }
    }
}
