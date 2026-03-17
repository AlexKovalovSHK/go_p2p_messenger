# Лог реализации (Implementation Status Log) — Aether

## [2026-03-17] Спринт 1: Инфраструктура — Начало
**Статус**: В процессе

### Задачи Спринта 1:
- [x] **Logging**: Создать этот файл и зафиксировать начало.
- [x] **Infrastructure (Event Bus)**: Пакет `internal/event` с методами `Publish` и `Subscribe`.
- [x] **Storage (SQLite)**: Новые миграции (`contacts`, `messages` update).
- [x] **Integration**: Внедрение шины в `main.go`.

## Спринт 2: Бизнес-логика
**Статус**: Завершено

### Задачи Спринта 2:
- [x] **Message Processing**: Реализация `ProcessIncoming(from, payload)` в `Processor`.
- [x] **Storage Integration**: Автоматическое сохранение входящих сообщений в БД.
- [x] **Event Propagation**: Публикация `chat.message.new` при получении.
- [x] **Peer Events**: Публикация событий онлайн/оффлайн при обнаружении пиров.

## Спринт 4: Мостик (API & Event Bus)
**Статус**: Завершено

### Задачи Спринта 4:
- [x] **Decryption**: `ChatService` автоматически дешифрует сообщения для UI.
- [x] **Identity Management**: `NodeService` поддерживает экспорт и генерацию ключей.
- [x] **Integration**: Полная готовность API-слоя к подключению Fyne GUI.
