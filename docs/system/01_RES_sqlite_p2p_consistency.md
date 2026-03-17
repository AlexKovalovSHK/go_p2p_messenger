# 01_RES_sqlite_p2p_consistency.md — SQLite в P2P-приложении Aether

**Статус:** Research / Draft  
**Дата:** 2026-03-17  
**Драйвер:** `modernc.org/sqlite` (pure Go, no CGO)

---

## Оглавление

1. [Concurrency: WAL mode и безопасная запись](#1-concurrency-wal-mode-и-безопасная-запись)
2. [Схема синхронизации: DeviceID и last_synced_id](#2-схема-синхронизации-deviceid-и-last_synced_id)
3. [PRAGMA настройки для 24/7 работы](#3-pragma-настройки-для-247-работы)
4. [Миграции без CGO](#4-миграции-без-cgo)
5. [Риски и рекомендации](#5-риски-и-рекомендации)

---

## 1. Concurrency: WAL mode и безопасная запись

### 1.1 Почему WAL (Write-Ahead Logging)

По умолчанию SQLite использует **rollback journal** — при любой записи вся база блокируется (exclusive lock). Это неприемлемо для Aether, где одновременно работают:

- P2P-потоки (приём сообщений от нескольких пиров)
- GUI-поток (Fyne — render loop)
- Фоновые задачи (синхронизация с Personal Node, очистка старых сообщений)

**WAL mode** решает проблему:

| Операция | Rollback Journal | WAL |
|---|---|---|
| Читатели блокируют писателей | Да | Нет |
| Писатель блокирует читателей | Да | Нет |
| Несколько параллельных читателей | Нет | Да |
| Несколько параллельных писателей | Нет | Нет* |

*\* SQLite поддерживает только одного писателя одновременно, но WAL сериализует их без блокировки читателей.*

### 1.2 Инициализация соединения с WAL

```go
package storage

import (
    "database/sql"
    "fmt"
    "sync"

    _ "modernc.org/sqlite" // pure Go драйвер, no CGO
)

// DB — тонкая обёртка над *sql.DB с гарантией единственного писателя.
type DB struct {
    rdb  *sql.DB // пул соединений только для чтения
    wdb  *sql.DB // единственное соединение для записи
    wmu  sync.Mutex // сериализация записей
}

// Open открывает базу данных с оптимальными настройками для P2P-приложения.
func Open(path string) (*DB, error) {
    // DSN с встроенными PRAGMA для modernc.org/sqlite
    dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)

    // Пул читателей: несколько горутин могут читать одновременно
    rdb, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open read pool: %w", err)
    }
    rdb.SetMaxOpenConns(4)           // WAL разрешает N читателей
    rdb.SetMaxIdleConns(4)
    rdb.SetConnMaxLifetime(0)        // соединения живут вечно (нет CGO-leak)

    // Одно соединение для записи — предотвращает "database is locked" (SQLITE_BUSY)
    wdb, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open write conn: %w", err)
    }
    wdb.SetMaxOpenConns(1)           // КРИТИЧНО: один писатель
    wdb.SetMaxIdleConns(1)

    // Применяем PRAGMA после открытия
    if err := applyPragmas(wdb); err != nil {
        return nil, err
    }

    return &DB{rdb: rdb, wdb: wdb}, nil
}

func applyPragmas(db *sql.DB) error {
    pragmas := []string{
        "PRAGMA journal_mode = WAL",
        "PRAGMA synchronous = NORMAL",    // Баланс надёжности и скорости
        "PRAGMA cache_size = -32000",     // 32 MB page cache
        "PRAGMA busy_timeout = 5000",     // 5 сек ожидания при SQLITE_BUSY
        "PRAGMA wal_autocheckpoint = 1000", // checkpoint каждые 1000 страниц
        "PRAGMA temp_store = MEMORY",     // временные таблицы в RAM
        "PRAGMA mmap_size = 134217728",   // 128 MB mmap для быстрого чтения
        "PRAGMA foreign_keys = ON",
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            return fmt.Errorf("pragma %q: %w", p, err)
        }
    }
    return nil
}

// Write сериализует все записи через единственный writer connection.
func (d *DB) Write(fn func(*sql.Tx) error) error {
    d.wmu.Lock()
    defer d.wmu.Unlock()

    tx, err := d.wdb.Begin()
    if err != nil {
        return err
    }
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    return tx.Commit()
}

// Query выполняет читающий запрос из пула читателей.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
    return d.rdb.Query(query, args...)
}
```

### 1.3 Паттерн использования в P2P-потоках

```go
// В P2P-обработчике входящих сообщений:
func (h *MessageHandler) HandleIncoming(msg *Message) error {
    return h.db.Write(func(tx *sql.Tx) error {
        _, err := tx.Exec(`
            INSERT OR IGNORE INTO messages (id, sender_id, content, received_at, device_id)
            VALUES (?, ?, ?, ?, ?)
        `, msg.ID, msg.SenderID, msg.Content, msg.ReceivedAt, msg.SourceDeviceID)
        return err
    })
}

// В GUI-потоке (Fyne) — только чтение, без блокировки P2P:
func (ui *ChatView) LoadMessages(conversationID string) ([]*Message, error) {
    rows, err := ui.db.Query(`
        SELECT id, sender_id, content, received_at
        FROM messages
        WHERE conversation_id = ?
        ORDER BY received_at DESC LIMIT 50
    `, conversationID)
    // ... обработка rows
}
```

---

## 2. Схема синхронизации: DeviceID и last_synced_id

### 2.1 Концепция Personal Node

Personal Node — это всегда-онлайн устройство пользователя (VPS, домашний сервер), выступающее:
- Relay для других устройств пользователя
- Хранилищем непрочитанных сообщений при оффлайн устройств
- Точкой синхронизации между устройствами

### 2.2 Схема базы данных

```sql
-- Устройства пользователя (его собственные девайсы + Personal Node)
CREATE TABLE devices (
    id          TEXT PRIMARY KEY,           -- Ed25519 PeerID устройства
    user_id     TEXT NOT NULL,              -- владелец
    name        TEXT NOT NULL,             -- "iPhone 15", "Personal Node"
    is_personal_node INTEGER DEFAULT 0,   -- флаг Personal Node
    last_seen_at INTEGER,                  -- Unix timestamp
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);

-- Сообщения с глобальным монотонным счётчиком
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,       -- UUID v4 / content-hash
    conversation_id TEXT NOT NULL,
    sender_id       TEXT NOT NULL,          -- PeerID отправителя
    source_device_id TEXT NOT NULL,         -- с какого устройства отправлено
    content         BLOB NOT NULL,          -- зашифрованный payload
    global_seq      INTEGER NOT NULL,       -- монотонный счётчик (AUTOINCREMENT)
    sent_at         INTEGER NOT NULL,       -- timestamp отправителя (Unix ms)
    received_at     INTEGER NOT NULL DEFAULT (unixepoch('now','subsec')*1000),
    status          TEXT DEFAULT 'received' -- 'received'|'delivered'|'read'
);

-- Прогресс синхронизации каждого устройства
-- Хранит, до какого global_seq устройство получило сообщения
CREATE TABLE device_sync_state (
    device_id       TEXT NOT NULL,          -- устройство-получатель
    conversation_id TEXT NOT NULL,          -- контекст (чат)
    last_synced_seq INTEGER NOT NULL DEFAULT 0, -- последний подтверждённый seq
    synced_at       INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (device_id, conversation_id),
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- Индексы для эффективной синхронизации
CREATE INDEX idx_messages_conv_seq
    ON messages (conversation_id, global_seq);

CREATE INDEX idx_messages_global_seq
    ON messages (global_seq);

-- Контакты (доверенные пиры)
CREATE TABLE contacts (
    peer_id         TEXT PRIMARY KEY,       -- Ed25519 PeerID
    display_name    TEXT,
    added_at        INTEGER NOT NULL DEFAULT (unixepoch()),
    last_known_addr TEXT                    -- последний multiaddress
);
```

### 2.3 Логика синхронизации с Personal Node

```go
package sync

import (
    "context"
    "database/sql"
    "encoding/json"

    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/libp2p/go-libp2p/core/protocol"
)

const SyncProtocolID = protocol.ID("/aether/sync/1.0.0")

// SyncRequest запрашивает сообщения после определённого seq.
type SyncRequest struct {
    DeviceID       string `json:"device_id"`
    ConversationID string `json:"conversation_id"`
    AfterSeq       int64  `json:"after_seq"`    // last_synced_seq
    Limit          int    `json:"limit"`
}

type SyncResponse struct {
    Messages []MessageDTO `json:"messages"`
    MaxSeq   int64        `json:"max_seq"`
}

// GetLastSyncedSeq читает последний подтверждённый seq для устройства.
func GetLastSyncedSeq(db *sql.DB, deviceID, conversationID string) (int64, error) {
    var seq int64
    err := db.QueryRow(`
        SELECT last_synced_seq FROM device_sync_state
        WHERE device_id = ? AND conversation_id = ?
    `, deviceID, conversationID).Scan(&seq)
    if err == sql.ErrNoRows {
        return 0, nil // устройство синхронизируется впервые
    }
    return seq, err
}

// UpdateSyncState обновляет прогресс после успешной доставки.
func UpdateSyncState(tx *sql.Tx, deviceID, conversationID string, maxSeq int64) error {
    _, err := tx.Exec(`
        INSERT INTO device_sync_state (device_id, conversation_id, last_synced_seq, synced_at)
        VALUES (?, ?, ?, unixepoch())
        ON CONFLICT (device_id, conversation_id)
        DO UPDATE SET last_synced_seq = excluded.last_synced_seq,
                      synced_at = excluded.synced_at
    `, deviceID, conversationID, maxSeq)
    return err
}

// GetMissedMessages возвращает сообщения, которые устройство пропустило.
func GetMissedMessages(db *sql.DB, conversationID string, afterSeq int64, limit int) ([]MessageDTO, error) {
    rows, err := db.Query(`
        SELECT id, sender_id, source_device_id, content, global_seq, sent_at
        FROM messages
        WHERE conversation_id = ? AND global_seq > ?
        ORDER BY global_seq ASC
        LIMIT ?
    `, conversationID, afterSeq, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var msgs []MessageDTO
    for rows.Next() {
        var m MessageDTO
        if err := rows.Scan(&m.ID, &m.SenderID, &m.SourceDeviceID,
            &m.Content, &m.GlobalSeq, &m.SentAt); err != nil {
            return nil, err
        }
        msgs = append(msgs, m)
    }
    return msgs, rows.Err()
}
```

### 2.4 Схема синхронизации (таймлайн)

```
Устройство B (оффлайн)    Personal Node     Устройство A
        |                      |                 |
        |                      |◄── msg #101 ────|
        |                      |◄── msg #102 ────|
   [B идёт онлайн]             |                 |
        |                      |                 |
        |── SyncRequest ───────►|                 |
        |   device_id=B         |                 |
        |   after_seq=100       |                 |
        |                      |                 |
        |◄── SyncResponse ──────|                 |
        |    msgs [#101, #102]  |                 |
        |    max_seq=102        |                 |
        |                      |                 |
   [B обновляет last_synced_seq=102]             |
        |── UpdateSyncState ────►|                |
```

---

## 3. PRAGMA настройки для 24/7 работы

### 3.1 Полный набор PRAGMA с обоснованием

```go
// PragmaConfig — все настройки SQLite для продакшн-режима Aether.
var PragmaConfig = []string{
    // WAL — обязательно для concurrency (R/W не блокируют друг друга)
    "PRAGMA journal_mode = WAL",

    // NORMAL: fsync только при checkpoint, не при каждом коммите.
    // Риск: потеря последнего коммита при сбое питания (приемлемо для мессенджера).
    // FULL — в 3-5 раз медленнее, избыточно.
    "PRAGMA synchronous = NORMAL",

    // 32 MB page cache (отрицательное значение = килобайты).
    // Снижает I/O при повторном чтении переписок.
    "PRAGMA cache_size = -32000",

    // 5 секунд ожидания при SQLITE_BUSY вместо немедленной ошибки.
    // Критично при конкуренции P2P-потоков за writer.
    "PRAGMA busy_timeout = 5000",

    // Автоматический WAL checkpoint каждые 1000 страниц (4 MB при page_size=4096).
    // Предотвращает бесконечный рост WAL-файла.
    "PRAGMA wal_autocheckpoint = 1000",

    // Временные таблицы (ORDER BY, GROUP BY) в памяти, не на диске.
    "PRAGMA temp_store = MEMORY",

    // 128 MB mmap для быстрого последовательного чтения истории.
    // Ускоряет загрузку переписки на ~2-3x vs read() syscalls.
    "PRAGMA mmap_size = 134217728",

    // Размер страницы 4096 байт — оптимален для SSD и современных FS.
    // Должен устанавливаться до первой записи данных!
    "PRAGMA page_size = 4096",

    // Каскадные удаления и проверка внешних ключей.
    "PRAGMA foreign_keys = ON",

    // Включить WAL2-совместимость (если поддерживается версией).
    // Позволяет параллельные checkpoint без блокировки.
    // "PRAGMA wal_checkpoint(TRUNCATE)", -- вызывать вручную при idle
}
```

### 3.2 Управление памятью для 24/7 приложения

```go
// PeriodicMaintenance — фоновая задача для поддержания здоровья БД.
func PeriodicMaintenance(ctx context.Context, db *DB) {
    // WAL checkpoint каждые 30 минут при простое
    checkpointTicker := time.NewTicker(30 * time.Minute)
    // ANALYZE каждые 6 часов (обновление статистики для query planner)
    analyzeTicker := time.NewTicker(6 * time.Hour)

    defer checkpointTicker.Stop()
    defer analyzeTicker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-checkpointTicker.C:
            // TRUNCATE: сжимает WAL-файл до нуля при отсутствии активных читателей
            db.wdb.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
        case <-analyzeTicker.C:
            db.wdb.Exec("ANALYZE")
        }
    }
}
```

### 3.3 Лимиты памяти

| PRAGMA | Значение | Память | Обоснование |
|---|---|---|---|
| `cache_size` | -32000 | 32 MB | Кэш частых чатов |
| `mmap_size` | 134217728 | 128 MB | Быстрое чтение истории |
| `temp_store` | MEMORY | ~10 MB peak | Сортировки в RAM |
| **Итого** | | **~170 MB** | Приемлемо для desktop |

Для мобильной версии: `cache_size = -8000` (8 MB), `mmap_size = 33554432` (32 MB).

---

## 4. Миграции без CGO

### 4.1 Рекомендуемые библиотеки

| Библиотека | CGO | Подход | Рекомендация |
|---|---|---|---|
| `golang-migrate/migrate` | ❌ No | SQL-файлы, embed | ✅ **Рекомендуем** |
| `pressly/goose` | ❌ No | SQL или Go | ✅ Альтернатива |
| Ручное управление | ❌ No | Кастомное | Для простых схем |

### 4.2 Реализация с golang-migrate + embed

```go
package storage

import (
    "embed"
    "fmt"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    "github.com/golang-migrate/migrate/v4/source/iofs"
    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations применяет все pending миграции.
// Безопасно вызывать при каждом старте приложения.
func RunMigrations(db *sql.DB) error {
    srcDriver, err := iofs.New(migrationsFS, "migrations")
    if err != nil {
        return fmt.Errorf("migrations source: %w", err)
    }

    dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{
        MigrationsTable: "schema_migrations",
    })
    if err != nil {
        return fmt.Errorf("migrations db driver: %w", err)
    }

    m, err := migrate.NewWithInstance("iofs", srcDriver, "sqlite", dbDriver)
    if err != nil {
        return fmt.Errorf("migrate init: %w", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migrate up: %w", err)
    }

    return nil
}
```

### 4.3 Структура файлов миграций

```
storage/
├── migrations/
│   ├── 000001_initial_schema.up.sql
│   ├── 000001_initial_schema.down.sql
│   ├── 000002_add_contacts.up.sql
│   ├── 000002_add_contacts.down.sql
│   └── 000003_add_sync_state.up.sql
│   └── 000003_add_sync_state.down.sql
└── db.go
```

```sql
-- migrations/000001_initial_schema.up.sql
CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id       TEXT NOT NULL,
    source_device_id TEXT NOT NULL,
    content         BLOB NOT NULL,
    global_seq      INTEGER NOT NULL,
    sent_at         INTEGER NOT NULL,
    received_at     INTEGER NOT NULL DEFAULT (unixepoch('now','subsec')*1000),
    status          TEXT DEFAULT 'received'
);

CREATE INDEX IF NOT EXISTS idx_messages_conv_seq
    ON messages (conversation_id, global_seq);
```

```sql
-- migrations/000001_initial_schema.down.sql
DROP INDEX IF EXISTS idx_messages_conv_seq;
DROP TABLE IF EXISTS messages;
```

### 4.4 Альтернатива: ручное управление schema_version

Если зависимость от `golang-migrate` нежелательна:

```go
// SimpleSchemaManager — ручное управление версиями схемы.
type SimpleSchemaManager struct {
    db *sql.DB
}

func (m *SimpleSchemaManager) Migrate() error {
    // Создать таблицу версий если не существует
    m.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`)

    var current int
    m.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)

    migrations := []struct {
        version int
        sql     string
    }{
        {1, schemaV1},
        {2, schemaV2},
        // добавлять новые версии здесь
    }

    for _, mig := range migrations {
        if mig.version <= current {
            continue
        }
        if _, err := m.db.Exec(mig.sql); err != nil {
            return fmt.Errorf("migration v%d: %w", mig.version, err)
        }
        m.db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, mig.version)
    }
    return nil
}
```

**Рекомендация:** Использовать `golang-migrate` — надёжнее, поддерживает rollback, хорошо протестирован. Для Aether это добавит минимальную зависимость без CGO.

---

## 5. Риски и рекомендации

### 5.1 Таблица рисков

| Риск | Причина | Вероятность | Митигация |
|---|---|---|---|
| SQLITE_BUSY при конкуренции писателей | Несколько горутин пишут одновременно | Высокая | Единственный writer connection + `busy_timeout=5000` |
| Рост WAL-файла | Долгие читающие транзакции блокируют checkpoint | Средняя | Ограничить TTL читающих транзакций, периодический TRUNCATE |
| Потеря данных при сбое питания | `synchronous=NORMAL` | Низкая | Приемлемо для мессенджера; для критик. данных → `FULL` |
| Дублирование сообщений при ресинке | Повторная доставка уже принятых | Высокая | `INSERT OR IGNORE` + уникальный `id` |
| Несогласованность `global_seq` между узлами | Разные Personal Nodes генерируют свои seq | Средняя | seq = локальный автоинкремент Personal Node; клиенты используют seq своего PN |
| Раздробленность WAL на медленных дисках | Частые маленькие транзакции | Средняя | Batching: группировать несколько P2P-сообщений в одну транзакцию |

### 5.2 Антипаттерны для Aether

```go
// ❌ ПЛОХО: несколько Open() соединений без координации
db1, _ := sql.Open("sqlite", path) // GUI
db2, _ := sql.Open("sqlite", path) // P2P handler
// Результат: SQLITE_BUSY / database is locked

// ✅ ХОРОШО: один DB объект, разделённый между компонентами
db := storage.Open(path)
guiHandler := gui.New(db)
p2pHandler := p2p.New(db)

// ❌ ПЛОХО: читать в цикле без лимита
rows, _ := db.Query("SELECT * FROM messages") // может вернуть миллионы строк

// ✅ ХОРОШО: пагинация + LIMIT
rows, _ := db.Query("SELECT * FROM messages WHERE conversation_id=? ORDER BY global_seq DESC LIMIT 50 OFFSET ?", convID, offset)

// ❌ ПЛОХО: BEGIN EXCLUSIVE для чтения
tx, _ := db.Begin()
tx.Exec("BEGIN EXCLUSIVE") // блокирует всех читателей в WAL-режиме излишне

// ✅ ХОРОШО: обычные читающие запросы без явных транзакций
rows, _ := db.Query(readQuery)
```

### 5.3 Рекомендации по production-конфигурации

1. **WAL mode** — включать сразу при первом открытии БД (нельзя менять при открытых соединениях)
2. **Единственный writer** — всегда, даже если кажется что "читаем"
3. **`busy_timeout = 5000`** — обязательно, иначе P2P-потоки получат ошибку вместо ожидания
4. **`INSERT OR IGNORE`** — для идемпотентности при ресинхронизации
5. **`global_seq` как INTEGER AUTOINCREMENT** — только на Personal Node, клиенты не генерируют seq
6. **Checkpoint при idle** — `PRAGMA wal_checkpoint(TRUNCATE)` в фоне каждые 30 мин
7. **Не использовать CGO** — `modernc.org/sqlite` полностью совместим с `database/sql` API

---

*Документ подготовлен для команды Aether. Актуальная версия modernc.org/sqlite — v1.29.x.*
