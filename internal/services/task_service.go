package services

import (
    "context"
    "time"

    "github.com/mariana-kep/yourtaskplanner/internal/logger"
    "github.com/mariana-kep/yourtaskplanner/internal/models"
    "github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
    "github.com/mariana-kep/yourtaskplanner/internal/cache"
    "github.com/mariana-kep/yourtaskplanner/internal/kafka"
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
    repo  postgres.TaskRepository
    cache cache.Cache
    kafka kafka.Producer
    log   *logger.Logger
}

func NewTaskService(repo postgres.TaskRepository, cache cache.Cache, kafka kafka.Producer, log *logger.Logger) TaskService {
    return &taskService{repo: repo, cache: cache, kafka: kafka, log: log}
}

func (s *taskService) CreateTask(ctx context.Context, t *models.Task) error {
    if err := s.repo.Create(ctx, t); err != nil {
        return err
    }
    _ = s.cache.InvalidateTasks(ctx, t.OwnerID)
    msg := map[string]interface{}{
        "task_id": t.ID,
        "owner_id": t.OwnerID,
        "title": t.Title,
    }
    _ = s.kafka.Publish(ctx, "tasks.created", msg)
    return nil
}

func (s *taskService) GetTasks(ctx context.Context, userID int64) ([]models.Task, error) {
    tasks, err := s.cache.GetTasks(ctx, userID)
    if err == nil && len(tasks) > 0 {
        return tasks, nil
    }
    tasks, err = s.repo.GetByUser(ctx, userID)
    if err != nil {
        return nil, err
    }
    _ = s.cache.SetTasks(ctx, userID, tasks, time.Minute*5)
    return tasks, nil
}

func (s *taskService) UpdateTask(ctx context.Context, t *models.Task) error {
    if err := s.repo.Update(ctx, t); err != nil {
        return err
    }
    _ = s.cache.InvalidateTasks(ctx, t.OwnerID)
    return nil
}

func (s *taskService) DeleteTask(ctx context.Context, id int64) error {
    // fetch owner for cache invalidation could be added
    return s.repo.Delete(ctx, id)
}

func (s *taskService) AssignTask(ctx context.Context, taskID, userID int64) error {
    if err := s.repo.Assign(ctx, taskID, userID); err != nil {
        return err
    }
    msg := map[string]interface{}{
        "task_id": taskID,
        "assigned_to": userID,
    }
    _ = s.kafka.Publish(ctx, "tasks.assigned", msg)
    return nil
}

func (s *taskService) GetShared(ctx context.Context) ([]models.Task, error) {
    return s.repo.GetShared(ctx)
}