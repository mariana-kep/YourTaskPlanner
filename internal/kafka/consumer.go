package kafka

import (
	"context"
	"encoding/json"
	"log"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/segmentio/kafka-go"
)

type TelegramNotification struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func ConsumeNotifications(broker, topic string, bot *tgbot.BotAPI) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		GroupID:  "notifications-consumer",
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer r.Close()
	ctx := context.Background()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Println("kafka read error:", err)
			continue
		}
		var n TelegramNotification
		if err := json.Unmarshal(m.Value, &n); err != nil {
			log.Println("unmarshal:", err)
			continue
		}
		msg := tgbot.NewMessage(n.ChatID, n.Text)
		_, err = bot.Send(msg)
		if err != nil {
			log.Println("telegram send:", err)
		}
	}
}
