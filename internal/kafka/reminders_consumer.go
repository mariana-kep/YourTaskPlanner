package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

func ConsumeRemindersAndPublishNotifications(broker string, producer Producer) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		GroupID:  "reminders-consumer",
		Topic:    "reminders.scheduled",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer r.Close()
	ctx := context.Background()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Println("kafka read reminders error:", err)
			time.Sleep(time.Second)
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(m.Value, &payload); err != nil {
			log.Println("unmarshal reminders:", err)
			continue
		}
		chatID := int64(0)
		if v, ok := payload["owner_id"].(float64); ok {
			chatID = int64(v)
		}
		title := ""
		if v, ok := payload["title"].(string); ok {
			title = v
		}
		desc := ""
		if v, ok := payload["description"].(string); ok {
			desc = v
		}
		text := fmt.Sprintf("⏰ Напоминание: %s\n\n%s", title, desc)
		notification := map[string]interface{}{
			"chat_id": chatID,
			"text":    text,
		}
		if err := producer.Publish(ctx, "notifications.telegram", notification); err != nil {
			log.Println("publish notifications.telegram:", err)
			continue
		}
	}
}
