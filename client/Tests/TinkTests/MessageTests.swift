import XCTest
import SwiftData
@testable import Tink

@MainActor
final class MessageTests: XCTestCase {
    func testMessageDefaultsAreUnreadAndUnstarred() {
        let message = TinkMessage(id: 1, title: "Title", body: "Body")
        XCTAssertFalse(message.isRead)
        XCTAssertFalse(message.isStarred)
    }

    func testStoredMessageRoundTrip() {
        let message = TinkMessage(id: 42, title: "Title", body: "Body", isRead: true, isStarred: true)
        let stored = StoredMessage(message: message)
        let result = stored.message()
        XCTAssertEqual(result.id, message.id)
        XCTAssertEqual(result.title, message.title)
        XCTAssertEqual(result.body, message.body)
        XCTAssertTrue(result.isRead)
        XCTAssertTrue(result.isStarred)
    }

    func testMessageStoreUsesInMemoryContainer() throws {
        let container = try ModelContainer(for: StoredMessage.self,
                                            configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        let store = MessageStore(container: container)
        store.upsert(TinkMessage(id: 1, title: "A", body: "B"))
        store.upsert(TinkMessage(id: 2, title: "C", body: "D", isStarred: true))
        XCTAssertEqual(store.fetchAll().count, 2)
        store.delete(ids: [1])
        XCTAssertEqual(store.fetchAll().map(\.id), [2])
        store.deleteAll()
        XCTAssertTrue(store.fetchAll().isEmpty)
    }

    func testMessageStoreCountsUnreadMessages() throws {
        let container = try ModelContainer(for: StoredMessage.self, configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        let store = MessageStore(container: container)
        store.upsert(TinkMessage(id: 1, title: "A", body: "B"))
        store.upsert(TinkMessage(id: 2, title: "C", body: "D", isRead: true))
        XCTAssertEqual(store.count(), 2)
        XCTAssertEqual(store.unreadCount(), 1)
    }

    func testSSEParsesCRLFAndMultilineData() {
        let block = "id: 9\r\nevent: notification\r\ndata: {\r\ndata:   \"id\": 7,\r\ndata:   \"title\": \"Hello\",\r\ndata:   \"body\": \"World\"\r\ndata: }"
        let message = SSEClient.message(from: block)
        XCTAssertEqual(message?.id, 7)
        XCTAssertEqual(message?.title, "Hello")
    }

    func testSSEIgnoresNonNotificationAndInvalidJSON() {
        XCTAssertNil(SSEClient.message(from: "event: ping\ndata: {}"))
        XCTAssertNil(SSEClient.message(from: "event: notification\ndata: invalid"))
    }

    func testMessageStorePreservesReadAndStarredOnUpdate() throws {
        let container = try ModelContainer(for: StoredMessage.self, configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        let store = MessageStore(container: container)
        store.upsert(TinkMessage(id: 1, title: "Old", body: "Old", isRead: true, isStarred: true))
        store.upsert(TinkMessage(id: 1, title: "New", body: "New"))
        let result = store.message(id: 1)
        XCTAssertEqual(result?.title, "New")
        XCTAssertTrue(result?.isRead == true)
        XCTAssertTrue(result?.isStarred == true)
    }

    func testMessageStoreReplaceRemovesStaleRecords() throws {
        let container = try ModelContainer(for: StoredMessage.self, configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        let store = MessageStore(container: container)
        store.upsert(TinkMessage(id: 1, title: "A", body: "A"))
        store.upsert(TinkMessage(id: 2, title: "B", body: "B"))
        store.replace(with: [TinkMessage(id: 2, title: "B2", body: "B2")])
        XCTAssertEqual(store.fetchAll().map(\.id), [2])
        XCTAssertEqual(store.message(id: 2)?.title, "B2")
    }

    func testTrimKeepsStarredMessagesWhenOverLimit() {
        let messages = (0..<5).map { TinkMessage(id: UInt64($0), title: "", body: "", isStarred: $0 == 0) }
        let trimmed = ConnectionManager.trimmed(messages, limit: 3)
        XCTAssertEqual(trimmed.count, 3)
        XCTAssertTrue(trimmed.contains { $0.id == 0 })
    }

    func testTrimKeepsExactlyTheLimit() {
        let messages = (0..<5000).map { TinkMessage(id: UInt64($0), title: "", body: "") }
        XCTAssertEqual(ConnectionManager.trimmed(messages).count, 5000)
    }

    func testDeletingUnknownMessageDoesNotChangeStore() throws {
        let container = try ModelContainer(for: StoredMessage.self, configurations: ModelConfiguration(isStoredInMemoryOnly: true))
        let store = MessageStore(container: container)
        store.upsert(TinkMessage(id: 1, title: "A", body: "B"))
        store.delete(ids: [999])
        XCTAssertEqual(store.count(), 1)
    }

    func testPingResponseDecodingRequiresServerFields() throws {
        let data = #"{"message":"pong","version":"0.2.0","build":"","st":1700000000000}"#.data(using: .utf8)!
        let response = try JSONDecoder().decode(APIClient.PingResponse.self, from: data)
        XCTAssertEqual(response.message, "pong")
        XCTAssertEqual(response.serverTime, 1700000000000)
    }
}
