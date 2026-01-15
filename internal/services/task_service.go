package services

import (
	"context"
	"time"

	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	"github.com/mariana-kep/yourtaskplanner/internal/models"
)

type TaskService interface {
	CreateTask(ctx context.Context, t *models.Task) error
	GetTasks(ctx context.Context, userID int64) ([]models.Task, error)
	UpdateTask(ctx context.Context, t *models.Task) error
	DeleteTask(ctx context.Context, id int64) error
	AssignTask(ctx context.Context, taskID, userID int64) error
	GetShared(ctx context.Context) ([]models.Task, error)
}

type taskService struct {
	repo  TaskRepository
	cache Cache
	kafka Producer
	log   *logger.Logger
}

func NewTaskService(repo TaskRepository, cache Cache, kafka Producer, log *logger.Logger) TaskService {
	return &taskService{repo: repo, cache: cache, kafka: kafka, log: log}
}

func (s *taskService) CreateTask(ctx context.Context, t *models.Task) error {
	if err := s.repo.Create(ctx, t); err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.InvalidateTasks(ctx, t.OwnerID)
	}
	if s.kafka != nil {
		msg := map[string]interface{}{
			"task_id":     t.ID,
			"owner_id":    t.OwnerID,
			"title":       t.Title,
			"description": t.Description,
		}
		_ = s.kafka.Publish(ctx, "tasks.created", msg)
	}
	return nil
}

func (s *taskService) GetTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	if s.cache != nil {
		if tasks, err := s.cache.GetTasks(ctx, userID); err == nil && len(tasks) > 0 {
			return tasks, nil
		}
	}
	tasks, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.SetTasks(ctx, userID, tasks, time.Minute*5)
	}
	return tasks, nil
}

func (s *taskService) UpdateTask(ctx context.Context, t *models.Task) error {
	var oldOwner int64
	if existing, err := s.repo.GetByID(ctx, t.ID); err == nil && existing != nil {
		oldOwner = existing.OwnerID
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return err
	}
	if s.cache != nil {
		if oldOwner != 0 {
			_ = s.cache.InvalidateTasks(ctx, oldOwner)
		}
		_ = s.cache.InvalidateTasks(ctx, t.OwnerID)
	}
	return nil
}

func (s *taskService) DeleteTask(ctx context.Context, id int64) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.cache != nil && t != nil {
		_ = s.cache.InvalidateTasks(ctx, t.OwnerID)
	}
	return nil
}

func (s *taskService) AssignTask(ctx context.Context, taskID, userID int64) error {
	t, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if err := s.repo.Assign(ctx, taskID, userID); err != nil {
		return err
	}
	if s.cache != nil && t != nil {
		_ = s.cache.InvalidateTasks(ctx, t.OwnerID)
		_ = s.cache.InvalidateTasks(ctx, userID)
	}
	if s.kafka != nil {
		msg := map[string]interface{}{
			"task_id":     taskID,
			"assigned_to": userID,
		}
		_ = s.kafka.Publish(ctx, "tasks.assigned", msg)
	}
	return nil
}

func (s *taskService) GetShared(ctx context.Context) ([]models.Task, error) {
	return s.repo.GetShared(ctx)
}
