import XCTest

@testable import Tink

final class URLProtocolTests: XCTestCase {
    func testAPIClientUsesHTTPErrorResponse() async {
        MockURLProtocol.setHandler { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!
            return (response, Data("unauthorized".utf8))
        }
        defer { MockURLProtocol.setHandler(nil) }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MockURLProtocol.self]
        let client = APIClient(session: URLSession(configuration: configuration))
        do {
            _ = try await client.registerCurrentDevice()
            XCTFail("Expected HTTP error")
        } catch {
            XCTAssertTrue(error.localizedDescription.contains("401"))
        }
    }

    func testHTTPStatusErrorsAreExposed() async {
        for status in [403, 500] {
            MockURLProtocol.setHandler { request in
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: status, httpVersion: nil, headerFields: nil)!
                return (response, Data("failure".utf8))
            }
            let configuration = URLSessionConfiguration.ephemeral
            configuration.protocolClasses = [MockURLProtocol.self]
            let client = APIClient(session: URLSession(configuration: configuration))
            do {
                _ = try await client.registerCurrentDevice()
                XCTFail("Expected HTTP \(status) error")
            } catch {
                XCTAssertTrue(error.localizedDescription.contains("\(status)"))
            }
        }
        MockURLProtocol.setHandler(nil)
    }
}

private final class MockURLProtocol: URLProtocol {
    private final class HandlerState: @unchecked Sendable {
        private let lock = NSLock()
        private var handler: ((URLRequest) -> (HTTPURLResponse, Data))?

        func set(_ handler: ((URLRequest) -> (HTTPURLResponse, Data))?) {
            lock.lock()
            defer { lock.unlock() }
            self.handler = handler
        }

        func get() -> ((URLRequest) -> (HTTPURLResponse, Data))? {
            lock.lock()
            defer { lock.unlock() }
            return handler
        }
    }

    private static let handlerState = HandlerState()

    static func setHandler(_ handler: ((URLRequest) -> (HTTPURLResponse, Data))?) {
        handlerState.set(handler)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let result = Self.handlerState.get()?(request) else { return }
        client?.urlProtocol(self, didReceive: result.0, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: result.1)
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
