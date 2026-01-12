package worker

import (
	"context"
	"time"

	"github.com/mariana-kep/yourtaskplanner/internal/kafka"
	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	"github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
)

func StartScheduler(repo postgres.TaskRepository, producer kafka.Producer, log *logger.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	ctx := context.Background()
	for range ticker.C {
		now := time.Now()
		tasks, err := repo.GetDueReminders(ctx, now)
		if err != nil {
			log.Error("scheduler get due reminders", "error", err)
			continue
		}
		for _, t := range tasks {
			payload := map[string]interface{}{
				"task_id":   t.ID,
				"owner_id":  t.OwnerID,
				"title":     t.Title,
				"remind_at": t.RemindAt,
			}
			if err := producer.Publish(ctx, "reminders.scheduled", payload); err != nil {
				log.Error("publish reminders.scheduled", "error", err)
				continue
			}
			_ = repo.MarkReminderScheduled(ctx, t.ID)
		}
	}
}
