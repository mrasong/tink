import Foundation
import SwiftData

@MainActor
public final class MessageStore {
    public private(set) var lastError: Error?
    public let container: ModelContainer
    private let context: ModelContext

    public init(container: ModelContainer) {
        self.container = container
        self.context = ModelContext(container)
    }

    public func saveErrorDescription() -> String? { lastError?.localizedDescription }

    public func savePending() {
        save()
    }

    public func fetchAll() -> [TinkMessage] {
        let request = FetchDescriptor<StoredMessage>(sortBy: [SortDescriptor(\StoredMessage.receivedAt, order: .reverse)])
        do {
            lastError = nil
            return try context.fetch(request).map { $0.message() }
        } catch {
            lastError = error
            return []
        }
    }

    public func upsert(_ message: TinkMessage) {
        do {
            if let object = try context.fetch(FetchDescriptor<StoredMessage>(predicate: #Predicate { $0.id == message.id })).first {
                let wasRead = object.isRead
                let wasStarred = object.isStarred
                object.update(from: message)
                object.isRead = wasRead
                object.isStarred = wasStarred
            } else { context.insert(StoredMessage(message: message)) }
            try context.save(); lastError = nil
        } catch { lastError = error }
    }

    public func replace(with messages: [TinkMessage]) {
        do {
            let retainedIDs = Set(messages.map(\.id))
            let objects = try context.fetch(FetchDescriptor<StoredMessage>())
            for object in objects where !retainedIDs.contains(object.id) { context.delete(object) }
            for message in messages {
                if let object = try context.fetch(FetchDescriptor<StoredMessage>(predicate: #Predicate { $0.id == message.id })).first {
                    let wasRead = object.isRead; let wasStarred = object.isStarred
                    object.update(from: message); object.isRead = wasRead; object.isStarred = wasStarred
                } else { context.insert(StoredMessage(message: message)) }
            }
            try context.save(); lastError = nil
        } catch { lastError = error }
    }

    public func count() -> Int {
        do { lastError = nil; return try context.fetchCount(FetchDescriptor<StoredMessage>()) }
        catch { lastError = error; return 0 }
    }

    public func unreadCount() -> Int {
        do { lastError = nil; return try context.fetchCount(FetchDescriptor<StoredMessage>(predicate: #Predicate { !$0.isRead })) }
        catch { lastError = error; return 0 }
    }

    public func delete(ids: Set<UInt64>) {
        do {
            let objects = try context.fetch(FetchDescriptor<StoredMessage>())
            objects.filter { ids.contains($0.id) }.forEach(context.delete)
            try context.save(); lastError = nil
        } catch { lastError = error }
    }

    public func message(id: UInt64) -> TinkMessage? {
        do {
            lastError = nil
            return try context.fetch(FetchDescriptor<StoredMessage>(predicate: #Predicate { $0.id == id })).first?.message()
        } catch { lastError = error; return nil }
    }

    public func deleteAll() {
        do {
            let objects = try context.fetch(FetchDescriptor<StoredMessage>())
            objects.forEach(context.delete)
            try context.save(); lastError = nil
        } catch { lastError = error }
    }

    public func setRead(_ id: UInt64, value: Bool) { update(id) { $0.isRead = value } }
    public func setStarred(_ id: UInt64, value: Bool) { update(id) { $0.isStarred = value } }
    public func markAllAsRead() {
        do {
            let objects = try context.fetch(FetchDescriptor<StoredMessage>())
            objects.forEach { $0.isRead = true }; save()
        } catch { lastError = error }
    }

    private func update(_ id: UInt64, _ body: (StoredMessage) -> Void) {
        do {
            guard let object = try context.fetch(FetchDescriptor<StoredMessage>(predicate: #Predicate { $0.id == id })).first else { return }
            body(object); save()
        } catch { lastError = error }
    }

    private func save() {
        do { try context.save(); lastError = nil } catch { lastError = error }
    }
}
