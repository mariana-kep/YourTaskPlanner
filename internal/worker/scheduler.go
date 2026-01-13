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
	defer ticker.Stop()

	for {
		now := time.Now().UTC()
		tasks, err := repo.GetDueReminders(context.Background(), now)
		if err != nil {
			log.Error("scheduler get due reminders error: %v", err)
		} else {
			for _, t := range tasks {
				msg := map[string]interface{}{
					"task_id":     t.ID,
					"owner_id":    t.OwnerID,
					"title":       t.Title,
					"description": t.Description,
				}
				if err := producer.Publish(context.Background(), "reminders.scheduled", msg); err != nil {
					log.Error("scheduler publish reminder error: %v", err)
					continue
				}
				if err := repo.MarkReminderScheduled(context.Background(), t.ID); err != nil {
					log.Error("scheduler mark scheduled error: %v", err)
				}
			}
		}
		<-ticker.C
	}
}
