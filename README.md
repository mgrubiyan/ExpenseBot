# 💸 ExpenseBot

🤖 **Попробовать живого бота: [@UrM0ney_bot](https://t.me/UrM0ney_bot)**

## О проекте

ExpenseBot — это Telegram-бот, который помогает отслеживать личные расходы прямо в мессенджере. Проект создан с целью изучения разработки Telegram-ботов на Go, работы с базами данных и организации структуры реального backend-приложения.

**Что изучалось в ходе разработки:**

- Интеграция с Telegram Bot API через библиотеку `go-telegram-bot-api`
- Проектирование интерфейса `Storage` для абстракции слоя данных
- Работа с двумя базами данных: SQLite (локально) и PostgreSQL (продакшн)
- Организация кода по принципу Go-проектов (`cmd/`, `internal/`)
- Деплой приложения через `Procfile` (Heroku/Railway)
- Управление конфигурацией через переменные окружения

## Возможности

| Команда / действие | Описание |
|---|---|
| `еда 450` | Добавить трату (категория + сумма) |
| `/today` | Расходы за сегодня |
| `/week` | Расходы за последние 7 дней |
| `/month` | Расходы за текущий месяц |
| `/l5` | Последние 5 трат |
| `/del` | Удалить последнюю трату |
| `/help` | Справка по командам |

Бот также поддерживает inline-кнопки (Inline Keyboards) для навигации по меню.

## Архитектура

```
ExpenseBot/
├── cmd/
│   └── expense-bot/
│       └── main.go          # Точка входа, инициализация зависимостей
└── internal/
    ├── bot/
    │   ├── bot.go           # Структура бота и запуск polling
    │   ├── handlers.go      # Обработка команд и callback-кнопок
    │   ├── actions.go       # Бизнес-логика (статистика, удаление)
    │   └── keyboard.go      # Inline-клавиатуры для меню
    ├── models/
    │   └── expense.go       # Модель расхода, парсинг ввода
    └── storage/
        ├── storage.go       # Интерфейс Storage (абстракция БД)
        ├── SQLite_storage.go   # Реализация для SQLite
        └── PostgresStorage.go  # Реализация для PostgreSQL
```

Ключевое архитектурное решение — интерфейс `Storage`, благодаря которому бот не зависит от конкретной базы данных. При запуске автоматически выбирается PostgreSQL (если задана переменная `DATABASE_URL`) или SQLite как fallback.

## Быстрый старт

### Требования

- Go 1.21+
- Telegram Bot Token (получить через [@BotFather](https://t.me/BotFather))

### Локальный запуск (SQLite)

```bash
# Клонировать репозиторий
git clone https://github.com/mgrubiyan/ExpenseBot.git
cd ExpenseBot

# Создать файл с переменными окружения
echo "TELEGRAM_BOT_TOKEN=your_token_here" > .env

# Запустить бота
go run ./cmd/expense-bot
```

Данные сохраняются в `data/expenses.db` автоматически.

### С PostgreSQL

```bash
# Добавить DATABASE_URL в .env
echo "DATABASE_URL=postgres://user:password@localhost:5432/expensebot" >> .env

go run ./cmd/expense-bot
```

### Переменные окружения

| Переменная         | Описание                     | Обязательная                  |
| ------------------ | ---------------------------- | ----------------------------- |
| TELEGRAM_BOT_TOKEN | Токен бота от @BotFather     | ✅                             |
| DATABASE_URL       | PostgreSQL connection string | ❌ (используется SQLite)       |
| DB_PATH            | Путь к SQLite файлу          | ❌ (default: data/expenses.db) |
## Технологии

- **[Go](https://golang.org/)** — основной язык
- **[go-telegram-bot-api v5](https://github.com/go-telegram-bot-api/telegram-bot-api)** — работа с Telegram Bot API
- **[modernc/sqlite](https://pkg.go.dev/modernc.org/sqlite)** — SQLite без CGO
- **[lib/pq](https://github.com/lib/pq)** — PostgreSQL драйвер
- **[godotenv](https://github.com/joho/godotenv)** — загрузка `.env` файлов

## Автор

**Matvey Grubiyan** — [GitHub](https://github.com/mgrubiyan)

***
