import XCTest
@testable import Tink

@MainActor
final class UITestContractTests: XCTestCase {
    func testMessageRetentionContract() {
        let messages = (0...5000).map { TinkMessage(id: UInt64($0), title: "", body: "", isStarred: $0 == 0) }
        let retained = ConnectionManager.trimmed(messages)
        XCTAssertEqual(retained.count, 5000)
        XCTAssertTrue(retained.contains { $0.id == 0 })
        XCTAssertFalse(retained.contains { $0.id == 5000 })
    }
}
