# Research: P2P to UI Bridge (Aether)

## Overview
The goal is to establish a robust mechanism for transferring incoming P2P messages (libp2p streams) to the Fyne GUI layer while ensuring thread safety, low latency, and efficient storage in SQLite.

## 1. Transmission Mechanisms: Channels vs. PubSub

### Go Channels (Native)
*   **Pros**: Simple, built-in, type-safe, high performance.
*   **Cons**: Tight coupling if not managed via interfaces. Harder to support multiple observers (e.g., UI + Logger + Notification Service) without a "fan-out" pattern.
*   **Best for**: Direct 1-to-1 plumbing between a processor and the primary UI event loop.

### PubSub (Event Bus)
*   **Pros**: Decouples producers (libp2p handlers) from consumers (UI viewmodels). Easy to add new listeners without changing the source code.
*   **Cons**: Potential overhead (though negligible for messengers). Loose typing unless using a structured wrapper.
*   **Best for**: Complex applications like Aether where multiple components (SyncEngine, Discovery, Chat) need to react to the same network event.

### Recommendation
Use a **Centralized Event Bus** (PubSub) using Go channels internally. Libp2p handlers publish events to the bus; the UI ViewModels subscribe only to relevant topics (e.g., `messages.new`, `peer.status`).

## 2. Preventing Data Races (SQLite + UI)

### The Challenge
Fyne UI runs on the main thread. P2P handlers run on separate goroutines. SQLite (modernc.org/sqlite) handles concurrency, but we must ensure:
1.  **UI Updates**: Occur on the main thread via `Refresh()` or data bindings.
2.  **Database Consistency**: The UI shouldn't read a message before the transaction that saves it is committed.

### Strategies
1.  **Single-Writer SQLite**: Use a dedicated goroutine or a mutex-wrapped repository for SQLite writes.
2.  **Event Order**: 
    - Incoming Message -> Libp2p Goroutine.
    - Save to SQLite -> Transaction Commits.
    - Publish "MessageSaved" Event -> EventBus.
    - UI Listener receives event -> Updates Data Binding -> Fyne schedules redraw.
3.  **Fyne Data Bindings**: Use `fyne.io/fyne/v2/data/binding` to abstract the thread safety. Bind the UI widgets to a `binding.UntypedList` that is updated on the main thread.

## 3. Implementation Workflow
- **Producer**: `internal/transport` triggers `internal/logic` (processor).
- **Bridge**: `logic.Processor` saves to `internal/storage` and publishes to `internal/event` bus.
- **Consumer**: `internal/ui` (ViewModel) listens to the bus and updates bindings.
