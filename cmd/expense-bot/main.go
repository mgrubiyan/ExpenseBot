package main

import (
	"log"
	"os"

	"ExpenseBot/internal/bot"
	"ExpenseBot/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("failed to load .env file")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	st, err := storage.NewSQLiteStorage("data/expenses.db")
	if err != nil {
		log.Fatal("failed to init storage:", err)
	}

	b, err := bot.New(token, st)
	if err != nil {
		log.Fatal("failed to init bot:", err)
	}

	if err := b.Run(); err != nil {
		log.Fatal("bot stopped with error:", err)
	}
}
