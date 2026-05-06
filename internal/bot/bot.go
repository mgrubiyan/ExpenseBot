package bot

import (
	"fmt"
	"log"

	"ExpenseBot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	handler *Handler
}

func New(token string, st storage.Storage) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}

	handler := NewHandler(st)

	return &Bot{
		api:     api,
		handler: handler,
	}, nil
}

func (b *Bot) Run() error {
	log.Printf("authorized as @%s", b.api.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := b.api.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		b.handler.HandleUpdate(b.api, update)
	}

	return nil
}
