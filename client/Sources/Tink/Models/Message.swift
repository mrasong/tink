import Foundation

public struct TinkMessage: Codable, Identifiable, Sendable {
    public let id: UInt64
    public let devices: [String]?
    public let group: String?
    public let title: String
    public let body: String
    public let url: String?
    public let sound: String?
    public let payload: [String: AnyCodable]?
    public let createdAt: Int64?
    public var isRead: Bool
    public var isStarred: Bool

    enum CodingKeys: String, CodingKey {
        case id
        case devices
        case group
        case title
        case body
        case url
        case sound
        case payload
        case createdAt = "created_at"
        case isRead
        case isStarred
    }

    public init(
        id: UInt64, devices: [String]? = nil, group: String? = nil,
        title: String, body: String, url: String? = nil, sound: String? = nil,
        payload: [String: AnyCodable]? = nil, createdAt: Int64? = nil, isRead: Bool = false,
        isStarred: Bool = false
    ) {
        self.id = id
        self.devices = devices
        self.group = group
        self.title = title
        self.body = body
        self.url = url
        self.sound = sound
        self.payload = payload
        self.createdAt = createdAt
        self.isRead = isRead
        self.isStarred = isStarred
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(UInt64.self, forKey: .id)
        devices = try c.decodeIfPresent([String].self, forKey: .devices)
        group = try c.decodeIfPresent(String.self, forKey: .group)
        title = try c.decode(String.self, forKey: .title)
        body = try c.decode(String.self, forKey: .body)
        url = try c.decodeIfPresent(String.self, forKey: .url)
        sound = try c.decodeIfPresent(String.self, forKey: .sound)
        payload = try c.decodeIfPresent([String: AnyCodable].self, forKey: .payload)
        createdAt = try c.decodeIfPresent(Int64.self, forKey: .createdAt)
        isRead = try c.decodeIfPresent(Bool.self, forKey: .isRead) ?? false
        isStarred = try c.decodeIfPresent(Bool.self, forKey: .isStarred) ?? false
    }
}

// AnyCodable 用于通用 JSON Payload 字段解析
public struct AnyCodable: Codable, @unchecked Sendable {
    public let value: Any

    public init(_ value: Any) {
        self.value = value
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let boolVal = try? container.decode(Bool.self) {
            value = boolVal
        } else if let intVal = try? container.decode(Int.self) {
            value = intVal
        } else if let doubleVal = try? container.decode(Double.self) {
            value = doubleVal
        } else if let stringVal = try? container.decode(String.self) {
            value = stringVal
        } else if let arrayVal = try? container.decode([AnyCodable].self) {
            value = arrayVal.map { $0.value }
        } else if let dictVal = try? container.decode([String: AnyCodable].self) {
            value = dictVal.mapValues { $0.value }
        } else {
            value = ""
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        if let b = value as? Bool {
            try container.encode(b)
        } else if let i = value as? Int {
            try container.encode(i)
        } else if let d = value as? Double {
            try container.encode(d)
        } else if let s = value as? String {
            try container.encode(s)
        } else {
            try container.encode("\(value)")
        }
    }
}
