import XCTest
@testable import Tink

@MainActor
final class UIAcceptanceTests: XCTestCase {
    func testRetentionAndStateContractsUsedByUI() {
        let unread = TinkMessage(id: 1, title: "Unread", body: "", isRead: false)
        let starred = TinkMessage(id: 2, title: "Starred", body: "", isStarred: true)
        XCTAssertFalse(unread.isRead)
        XCTAssertTrue(starred.isStarred)
        XCTAssertEqual(ConnectionManager.trimmed([unread, starred]).count, 2)
    }
}
