package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
	"github.com/segmentio/kafka-go"
)

type TelegramNotification struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func ConsumeNotifications(broker, topic string, bot *tgbot.BotAPI, repo postgres.TaskRepository, autoDelete bool) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		GroupID:  "notifications-consumer",
		Topic:    topic,
		MinBytes: 1e3,
		MaxBytes: 1e6,
	})
	defer r.Close()
	ctx := context.Background()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Println("kafka read notifications error:", err)
			time.Sleep(time.Second)
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(m.Value, &payload); err != nil {
			log.Println("invalid notification payload:", err)
			continue
		}
		var chatID int64
		if v, ok := payload["chat_id"].(float64); ok {
			chatID = int64(v)
		}
		text, _ := payload["text"].(string)
		taskID := int64(0)
		if v, ok := payload["task_id"].(float64); ok {
			taskID = int64(v)
		}

		msg := tgbot.NewMessage(chatID, text)
		if _, err := bot.Send(msg); err != nil {
			log.Println("telegram send error:", err)
			continue
		}

		if taskID != 0 {
			if err := repo.MarkReminderSent(ctx, taskID); err != nil {
				log.Println("mark reminder_sent error:", err)
			} else {
				log.Printf("marked task %d reminder_sent=true\n", taskID)
			}
			if autoDelete {
				if err := repo.Delete(ctx, taskID); err != nil {
					log.Println("delete task after reminder error:", err)
				} else {
					log.Printf("deleted task %d after reminder\n", taskID)
				}
			}
		}
	}
}
