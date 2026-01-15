package services

import (
	"context"
	"time"

	"github.com/mariana-kep/yourtaskplanner/internal/models"
)

type TaskRepository interface {
	Create(ctx context.Context, t *models.Task) error
	GetByUser(ctx context.Context, userID int64) ([]models.Task, error)
	Update(ctx context.Context, t *models.Task) error
	Delete(ctx context.Context, id int64) error
	Assign(ctx context.Context, taskID, userID int64) error
	GetShared(ctx context.Context) ([]models.Task, error)
	GetDueReminders(ctx context.Context, before time.Time) ([]models.Task, error)
	MarkReminderScheduled(ctx context.Context, taskID int64) error
	MarkReminderSent(ctx context.Context, taskID int64) error
	GetByID(ctx context.Context, id int64) (*models.Task, error)
}

type Cache interface {
	GetTasks(ctx context.Context, userID int64) ([]models.Task, error)
	SetTasks(ctx context.Context, userID int64, tasks []models.Task, ttl time.Duration) error
	InvalidateTasks(ctx context.Context, userID int64) error
}

type Producer interface {
	Publish(ctx context.Context, topic string, msg interface{}) error
}
