# Research: Fyne v2 Reactive UI for Messenger

## Overview
Fyne v2.4+ provides powerful data binding capabilities that allow creating reactive interfaces. This document explores how to implement core Aether features using these tools.

## 1. Auto-updating Chat List
Instead of manually refreshing the list, we bind the list widget to a `binding.ExternalUntypedList` or a managed `binding.UntypedList`.

### Implementation
1.  **ViewModel**: Maintains a `binding.UntypedList` of conversation summaries.
2.  **Watcher**: A background goroutine in the UI layer listens to the `EventBus` for `EventMessageReceived` or `EventContactNew`.
3.  **Binding**: When an event arrives, the VM fetches the latest list from SQLite and calls `Conversations.Set()`.
4.  **UI**: `widget.NewListWithData` automatically reflects changes when the binding is updated.

## 2. Dynamic Scroll to Bottom
Messengers require the view to scroll down automatically when a new message is sent or received.

### Implementation
- **Fyne Widget**: `container.NewVScroll` or `widget.NewList`.
- **Logic**: Use `list.ScrollToBottom()` after a message is added to the binding.
- **Timing**: Ensure the scroll call happens *after* the UI has finished rendering the new item (using `fyne.CurrentApp().Driver().CanvasForObject(o).Refresh(o)` or a slight defer).

## 3. Peer Status: Online/Offline Indicators
Libp2p events (Identification, Connection/Disconnection) must be reflected in the UI.

### Implementation
1.  **Transport Monitor**: `internal/transport.SetupAutoNAT` and `host.Network().Notify()` capture connectivity events.
2.  **Events**: Publish `EventPeerConnected` and `EventPeerDisconnected`.
3.  **UI Component**: Every chat item in the list has a small `canvas.Circle` bound to a `binding.Bool`.
4.  **Reactive Update**: The VM updates the `Bool` binding based on the `PeerID` found in the event.

## 4. Modern UI Aesthetics (Premium Look)
To avoid the "Default Tool" look:
- **Themes**: Override `fyne.Theme` to implement custom colors (Aether Purple/Dark Navy) and rounded message bubbles.
- **Layouts**: Use `container.NewGridWithRows` and `layout.NewSpacer` for a modern "Mobile-like" desktop experience.
- **Performance**: Use `modernc.org/sqlite` with `WAL` mode to ensure the UI remains responsive during heavy sync operations.
