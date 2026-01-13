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

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) Create(ctx context.Context, t *models.Task) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}
func (m *MockRepo) GetByUser(ctx context.Context, userID int64) ([]models.Task, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}
func (m *MockRepo) Update(ctx context.Context, t *models.Task) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}
func (m *MockRepo) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockRepo) Assign(ctx context.Context, taskID, userID int64) error {
	args := m.Called(ctx, taskID, userID)
	return args.Error(0)
}
func (m *MockRepo) GetShared(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}
func (m *MockRepo) GetDueReminders(ctx context.Context, before time.Time) ([]models.Task, error) {
	args := m.Called(ctx, before)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}
func (m *MockRepo) MarkReminderScheduled(ctx context.Context, taskID int64) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
}
func (m *MockRepo) MarkReminderSent(ctx context.Context, taskID int64) error {
	args := m.Called(ctx, taskID)
	return args.Error(0)
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

func (m *MockRepo) GetByID(ctx context.Context, id int64) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

type MockKafka struct {
	mock.Mock
}

func (m *MockKafka) Publish(ctx context.Context, topic string, msg interface{}) error {
	args := m.Called(ctx, topic, msg)
	return args.Error(0)
}

type TaskServiceSuite struct {
	suite.Suite
	repo  *MockRepo
	cache *MockCache
	kafka *MockKafka
	svc   services.TaskService
}

func (s *TaskServiceSuite) SetupTest() {
	s.repo = &MockRepo{}
	s.cache = &MockCache{}
	s.kafka = &MockKafka{}
	s.svc = services.NewTaskService(s.repo, s.cache, s.kafka, nil)
}

func (s *TaskServiceSuite) TestCreateTaskCallsRepoAndPublishes() {
	ctx := context.Background()
	t := &models.Task{Title: "hello", OwnerID: 1}
	s.repo.On("Create", ctx, t).Return(nil)
	s.cache.On("InvalidateTasks", ctx, int64(1)).Return(nil)
	s.kafka.On("Publish", ctx, "tasks.created", mock.Anything).Return(nil)
	err := s.svc.CreateTask(ctx, t)
	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Create", ctx, t)
	s.kafka.AssertCalled(s.T(), "Publish", ctx, "tasks.created", mock.Anything)
}

func TestTaskServiceSuite(t *testing.T) {
	suite.Run(t, new(TaskServiceSuite))
}
