# Проектирование: Логика чатов и БД (Aether)

## 1. Идентификация чатов (Chat ID)
В Aether каждый диалог идентифицируется уникальным `chat_id`. 
- Для **Direct Chat** (личные сообщения): `chat_id` равен `PeerID` собеседника (строка libp2p).
- Это позволяет мгновенно связывать входящий пакет (который всегда содержит `From: PeerID`) с конкретной веткой переписки.

## 2. Структура таблиц SQLite

### Таблица `contacts`
Хранит информацию о пользователях, с которыми было взаимодействие.
```sql
CREATE TABLE contacts (
    peer_id TEXT PRIMARY KEY,    -- libp2p PeerID
    alias TEXT,                  -- Отображаемое имя (опционально)
    public_key BLOB NOT NULL,     -- Ключ для шифрования (X25519)
    last_seen INTEGER,           -- Unix timestamp последнего онлайна
    is_trusted BOOLEAN DEFAULT 0 -- Флаг доверия (для PEX)
);
```

### Таблица `messages`
Хранит историю переписки.
```sql
CREATE TABLE messages (
    id TEXT PRIMARY KEY,         -- UUID сообщения
    chat_id TEXT NOT NULL,       -- PeerID собеседника
    sender_id TEXT NOT NULL,     -- PeerID отправителя (свой или чужой)
    content BLOB NOT NULL,       -- Зашифрованное или расшифрованное содержимое
    is_incoming BOOLEAN NOT NULL,-- 1 если получено, 0 если отправлено
    timestamp INTEGER NOT NULL,  -- Unix timestamp
    status TEXT DEFAULT 'sent',  -- sent, delivered, read
    FOREIGN KEY(chat_id) REFERENCES contacts(peer_id)
);
```

## 3. Алгоритм обработки сообщения от нового PeerID
Когда `MessageProcessor` получает данные от неизвестного `From: PeerID`:

1.  **Проверка контакта**: Выполняется `SELECT` в `contacts`. Если пусто:
2.  **Авто-создание контакта**: 
    - Из libp2p потока извлекается публичный ключ пира.
    - В таблицу `contacts` вставляется новая запись с `alias = "Unknown (" + shortID + ")"`.
3.  **Сохранение сообщения**: Сообщение вставляется в `messages` с `chat_id = From: PeerID`.
4.  **Уведомление**: Шина событий транслирует `EventContactNew` и `EventMessageReceived`. UI реагирует созданием новой строки в списке чатов.

## 4. Индексация для производительности
```sql
CREATE INDEX idx_messages_chat_time ON messages(chat_id, timestamp);
```
Это ускоряет пагинацию сообщений при открытии конкретного чата.
