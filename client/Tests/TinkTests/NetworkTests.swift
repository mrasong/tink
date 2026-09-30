import XCTest
@testable import Tink

final class NetworkTests: XCTestCase {
    func testPingResponseDecoding() throws {
        let data = #"{"message":"pong","version":"0.2.0","build":"dev","st":1700000000000}"#.data(using: .utf8)!
        let response = try JSONDecoder().decode(APIClient.PingResponse.self, from: data)
        XCTAssertEqual(response.message, "pong")
        XCTAssertEqual(response.version, "0.2.0")
        XCTAssertGreaterThan(response.serverTime, 0)
    }

    func testAPIEnvelopeErrorShape() throws {
        let data = #"{"code":403,"message":"forbidden","data":null}"#.data(using: .utf8)!
        let response = try JSONDecoder().decode(APIClient.APIResponse<Device>.self, from: data)
        XCTAssertEqual(response.code, 403)
        XCTAssertNil(response.data)
    }

    func testSSEDisconnectDisablesReconnect() {
        let client = SSEClient()
        client.disconnect()
        XCTAssertTrue(client.isStoppedForTesting)
    }

    func testSSEParsesHeartbeatAsNoMessage() {
        XCTAssertNil(SSEClient.message(from: ": heartbeat\n"))
    }

    func testSSERejectsMissingNotificationData() {
        XCTAssertNil(SSEClient.message(from: "event: notification\nid: 10"))
    }

    func testSSEDisconnectIsIdempotent() {
        let client = SSEClient()
        client.disconnect()
        client.disconnect()
        XCTAssertTrue(client.isStoppedForTesting)
    }
}
