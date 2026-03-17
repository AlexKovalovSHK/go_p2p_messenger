Aether — Design Documents (Phase 02)
Документы проектирования
 
02_DES_architecture_v1.md
 — Слои, интерфейсы, жизненный цикл
 Mermaid-диаграмма слоёв приложения
 Ключевые Go-интерфейсы (MessageTransport, Storage, SyncEngine, IdentityManager)
 Жизненный цикл запуска (Init sequence)
 
02_DES_sync_protocol.md
 — Протокол синхронизации
 Protobuf-структуры сообщений
 Процесс Handshake (Ed25519-авторизация)
 Алгоритм Fetch (global_seq-based)
 
02_DES_ui_ux_fyne.md
 — UI/UX архитектура Fyne
 Observable-модель (channels/pubsub)
 Экран Identity Manager
 Экран Chat List
 Экран Direct Chat
Документы планирования
 
03_PLN_implementation_phases.md
 — 5 спринтов
 
03_PLN_test_cases.md
 — критерии приёмки и тест-кейсы
Реализация (Sprint 1 - Основание)
 Scaffolding
 go mod init и загрузка зависимостей
 Структура директорий
 
Makefile
 (test, build, lint)
 internal/identity
 Интерфейс IdentityManager
 Генерация Ed25519 (go-libp2p)
 Сохранение/загрузка с правами 0600
 Sign / Verify
 Юнит-тесты (AC-S1-01..03)
 internal/storage (SQLite)
 Подключение БД (WAL + Pragma) + One-writer pool
 Миграции (golang-migrate + embed.FS) 000001_initial_schema.up.sql
 Репозитории: MessageRepository, ContactRepository, DeviceSyncRepository
 Smoke Test
 cmd/aether/main.go
 Исходный запуск БД, миграций, Identity и вывод DeviceID
Реализация (Sprint 2 - Сеть)
 internal/transport — интерфейсы
 MessageTransport, PeerDiscovery, NetworkReachability
 internal/transport — Libp2pTransport
 NewHost: QUIC + TCP + Noise + yamux + connmgr
 Hole punching (DCUTR) и NATPortMap
 Send / Subscribe реализация
 internal/transport — Discovery
 Dual DHT (WAN + LAN)
 mDNS (StartMDNS)
 AutoNAT (события о статусе в канал)
 internal/transport — PEX
 Обработчик /aether/pex/1.0.0
 Взаимодействие с доверенными контактами
 Тестирование (AC-S2-01..08)
 MockTransport
 Интеграционные тесты (mDNS, Send)
Реализация (Sprint 3 - Синхронизация)
 proto/aether/sync.proto
 Определение Protobuf сообщений
 Генерация Go-кода (protoc)
 internal/sync — Handshake
 Серверная часть (PersonalNodeServer)
 Клиентская часть (SyncClient)
 Верификация подписи (Ed25519)
 internal/sync — Fetch & Push
 Алгоритм пагинации по global_seq
 Real-time Push через стрим
 Подтверждения (ACK)
 internal/logic — Message Processor
 Шифрование (X25519 + ChaCha20Poly1305)
 Валидация подписи отправителя
 Тестирование (AC-S3-01..10)
 Тесты Handshake
 Тесты Fetch/Batch
 Сценарный тест real-time Push
Реализация (Sprint 4 — Мостик: API & Event Bus)
 internal/event — EventBus
 Channel-based bus (Publish/Subscribe)
 Автоматическая отписка по ctx.Done()
 Интеграция EventBus
 MessageProcessor -> EventMessageReceived (via SyncEngine/SendMessage)
 Transport -> EventNodeReachability
 SyncEngine -> EventSyncCompleted
 internal/api — Сервисы
 ChatService: ListConversations, GetMessages, SendMessage
 NodeService: GetStatus, SetPersonalNode, Identity Management
 Тестирование (AC-S4-01..06)
 Тесты EventBus (latency, non-blocking)
 Интеграционные тесты API + MockTransport
Реализация (Sprint 5 — Визуализация: UI Layer)
 internal/ui — Основа
 AppNavigator: Стек-навигация и переключение экранов
 Дизайн-система: Цвета, шрифты, стили пузырей
 Экран Identity Manager
 Отображение DeviceID и статус ноды
 Импорт/Экспорт ключей
 Настройка Personal Node
 Экран Chat List
 Список чатов с привязкой (binding)
 Последнее сообщение и счетчик непрочитанных
 Экран Direct Chat
 Пузыри сообщений с выравниванием (свой/чужой)
 Статусы доставки (⟳ ✓ ✓✓)
 Автопрокрутка вниз
 Финальная сборка
 Интеграция в main.go
 Проверка всех сценариев AC-S5-01..07