# Спецификация: Шина событий (Event Bus Spec)

## 1. Роль Event Bus
Шина событий служит "клеем" между сетевым слоем (libp2p) и интерфейсом (Fyne), гарантируя отсутствие прямых зависимостей.

## 2. Список системных событий

### `EventPeerOnline`
Генерируется, когда libp2p устанавливает соединение с пиром.
```go
type PeerEvent struct {
    PeerID    string
    Multiaddr string
}
```

### `EventPeerOffline`
Генерируется при разрыве соединения.
```go
type PeerEvent struct {
    PeerID string
}
```

### `EventNewMessage`
Генерируется при успешной записи входящего сообщения в базу данных.
```go
type MessageEvent struct {
    ID             string
    ChatID         string
    SenderID       string
    Text           string
    Timestamp      int64
    IsIncoming     bool
}
```

### `EventMessageDelivered`
Генерируется, когда получен ACK от удаленной стороны.
```go
type StatusEvent struct {
    MessageID string
    NewStatus string // "delivered", "read"
}
```

### `EventSyncProgress`
Для отображения прогресса синхронизации с Personal Node.
```go
type SyncEvent struct {
    Current int
    Total   int
    Status  string
}
```

## 3. Структура шины (Internal)
Шина использует `map[EventType][]chan Event` для подписки.

**Пример публикации:**
```go
bus.Publish("message.new", MessageEvent{...})
```

**Пример подписки в UI:**
```go
sub := bus.Subscribe("message.new")
go func() {
    for ev := range sub {
        msg := ev.(MessageEvent)
        if msg.ChatID == currentChatID {
            updateList(msg)
        }
    }
}()
```

## 4. Типизация
Все события упаковываются в общую структуру:
```go
type Event struct {
    Type EventType
    Data interface{}
}
```
Типы `EventType` определены как константы (string).
