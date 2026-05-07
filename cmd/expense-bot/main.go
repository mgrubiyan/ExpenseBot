package main

import (
	"log"
	"os"
	"path/filepath"

	"ExpenseBot/internal/bot"
	"ExpenseBot/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	var st storage.Storage
	var err error

	if pgURL := os.Getenv("DATABASE_URL"); pgURL != "" {
		log.Println("using Postgres storage")
		st, err = storage.NewPostgresStorage(pgURL)
		if err != nil {
			log.Fatal("failed to init postgres storage:", err)
		}
	} else {
		dbPath := os.Getenv("DB_PATH")
		if dbPath == "" {
			dbPath = "data/expenses.db"
		}

		dbDir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			log.Fatal("failed to create db directory:", err)
		}

		log.Println("using sqlite db:", dbPath)

		st, err = storage.NewSQLiteStorage(dbPath)
		if err != nil {
			log.Fatal("failed to init sqlite storage:", err)
		}
	}

	b, err := bot.New(token, st)
	if err != nil {
		log.Fatal("failed to init bot:", err)
	}

	if err := b.Run(); err != nil {
		log.Fatal("bot stopped with error:", err)
	}
}
