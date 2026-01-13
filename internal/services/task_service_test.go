package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
)

type MockTaskRepo struct {
	mock.Mock
}

func (m *MockTaskRepo) Create(ctx context.Context, t *models.Task) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTaskRepo) GetByUser(ctx context.Context, userID int64) ([]models.Task, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepo) Update(ctx context.Context, t *models.Task) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockTaskRepo) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepo) Assign(ctx context.Context, taskID, userID int64) error {
	args := m.Called(ctx, taskID, userID)
	return args.Error(0)
}

func (m *MockTaskRepo) GetShared(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepo) GetDueReminders(ctx context.Context, before time.Time) ([]models.Task, error) {
	args := m.Called(ctx, before)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepo) MarkReminderScheduled(ctx context.Context, taskID int64) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

func (m *MockTaskRepo) MarkReminderSent(ctx context.Context, taskID int64) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}

// New method added to interface — implement it in mock
func (m *MockTaskRepo) GetByID(ctx context.Context, id int64) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) GetTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockCache) SetTasks(ctx context.Context, userID int64, tasks []models.Task, ttl time.Duration) error {
	args := m.Called(ctx, userID, tasks, ttl)
	return args.Error(0)
}

func (m *MockCache) InvalidateTasks(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) Publish(ctx context.Context, topic string, msg interface{}) error {
	args := m.Called(ctx, topic, msg)
	return args.Error(0)
}

type TaskServiceSuite struct {
	suite.Suite
	repo  *MockTaskRepo
	cache *MockCache
	prod  *MockProducer
	svc   services.TaskService
}

func (s *TaskServiceSuite) SetupTest() {
	s.repo = &MockTaskRepo{}
	s.cache = &MockCache{}
	s.prod = &MockProducer{}
	s.svc = services.NewTaskService(s.repo, s.cache, s.prod, nil)
}

func (s *TaskServiceSuite) TestCreateTask_PublishesAndInvalidatesCache() {
	ctx := context.Background()
	t := &models.Task{
		ID:      1,
		OwnerID: 42,
		Title:   "Test task",
	}

	s.repo.On("Create", mock.Anything, t).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, t.OwnerID).Return(nil)
	s.prod.On("Publish", mock.Anything, "tasks.created", mock.MatchedBy(func(m interface{}) bool {
		payload, ok := m.(map[string]interface{})
		if !ok {
			return false
		}
		if payload["owner_id"] == nil || payload["task_id"] == nil {
			return false
		}
		return true
	})).Return(nil)

	err := s.svc.CreateTask(ctx, t)
	s.Require().NoError(err)

	s.repo.AssertExpectations(s.T())
	s.cache.AssertExpectations(s.T())
	s.prod.AssertExpectations(s.T())
}

func TestTaskServiceSuite(t *testing.T) {
	suite.Run(t, new(TaskServiceSuite))
}
