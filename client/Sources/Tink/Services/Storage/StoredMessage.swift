import Foundation
import SwiftData

@Model
public final class StoredMessage {
    @Attribute(.unique) public var id: UInt64
    public var devices: [String]?
    public var group: String?
    public var title: String
    public var body: String
    public var url: String?
    public var sound: String?
    public var payloadData: Data?
    public var createdAt: Int64?
    public var receivedAt: Int64
    public var isRead: Bool
    public var isStarred: Bool

    init(message: TinkMessage) {
        id = message.id; devices = message.devices; group = message.group
        title = message.title; body = message.body; url = message.url; sound = message.sound
        payloadData = try? JSONEncoder().encode(message.payload); createdAt = message.createdAt
        receivedAt = Int64(Date().timeIntervalSince1970 * 1000); isRead = message.isRead; isStarred = message.isStarred
    }

    func update(from message: TinkMessage) {
        devices = message.devices; group = message.group; title = message.title
        body = message.body; url = message.url; sound = message.sound
        payloadData = try? JSONEncoder().encode(message.payload); createdAt = message.createdAt
    }

    func message() -> TinkMessage {
        TinkMessage(id: id, devices: devices, group: group, title: title, body: body,
                    url: url, sound: sound, payload: payloadData.flatMap { try? JSONDecoder().decode([String: AnyCodable].self, from: $0) },
                    createdAt: createdAt, isRead: isRead, isStarred: isStarred)
    }
}
