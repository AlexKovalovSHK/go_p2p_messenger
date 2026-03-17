# Лог реализации (Implementation Status Log) — Aether

## [2026-03-17] Спринт 1: Инфраструктура — Начало
**Статус**: В процессе

### Задачи Спринта 1:
- [x] **Logging**: Создать этот файл и зафиксировать начало.
- [x] **Infrastructure (Event Bus)**: Пакет `internal/event` с методами `Publish` и `Subscribe`.
- [x] **Storage (SQLite)**: Новые миграции (`contacts`, `messages` update).
- [x] **Integration**: Внедрение шины в `main.go`.

---
*Заметки*: Переходим на строковые топики по запросу пользователя для упрощения отладки в UI.
