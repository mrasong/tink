import Foundation

public struct Device: Codable, Identifiable {
    public let id: String
    public var name: String
    public let createdAt: Int64?
    public var status: UInt8
    public var lastConnectedAt: Int64?
    public var lastDisconnectedAt: Int64?

    enum CodingKeys: String, CodingKey {
        case id
        case name
        case createdAt = "created_at"
        case status
        case lastConnectedAt = "last_connected_at"
        case lastDisconnectedAt = "last_disconnected_at"
    }

    public init(id: String, name: String, status: UInt8 = 0, createdAt: Int64? = nil,
                lastConnectedAt: Int64? = nil, lastDisconnectedAt: Int64? = nil) {
        self.id = id
        self.name = name
        self.status = status
        self.createdAt = createdAt
        self.lastConnectedAt = lastConnectedAt
        self.lastDisconnectedAt = lastDisconnectedAt
    }
}
