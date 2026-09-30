import XCTest
@testable import Tink

final class APIClientTests: XCTestCase {
    func testPingResponseRejectsMissingRequiredFields() {
        let data = #"{"message":"pong","version":"","build":"","st":0}"#.data(using: .utf8)!
        let response = try? JSONDecoder().decode(APIClient.PingResponse.self, from: data)
        XCTAssertEqual(response?.message, "pong")
        XCTAssertTrue(response?.version.isEmpty == true)
        XCTAssertEqual(response?.serverTime, 0)
    }

    func testEnvelopeDecodesSuccessAndFailureShapes() throws {
        let success = #"{"code":0,"message":"ok","data":{"id":"d","key_id":"sk-test","name":"Mac","status":0,"created_at":1789442541815,"last_connected_at":1789442541815,"last_disconnected_at":0}}"#.data(using: .utf8)!
        let response = try JSONDecoder().decode(APIClient.APIResponse<Device>.self, from: success)
        XCTAssertEqual(response.code, 0)
        XCTAssertEqual(response.data?.id, "d")

        let date = response.data?.createdAt
        XCTAssertNotNil(date)

        let failure = #"{"code":401,"message":"unauthorized","data":null}"#.data(using: .utf8)!
        let failed = try JSONDecoder().decode(APIClient.APIResponse<Device>.self, from: failure)
        XCTAssertEqual(failed.code, 401)
        XCTAssertNil(failed.data)
    }
}
