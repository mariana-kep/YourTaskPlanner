package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
	"github.com/mariana-kep/yourtaskplanner/internal/services/mocks"
)

type TaskServiceSuite struct {
	suite.Suite
	repo  *mocks.TaskRepository
	cache *mocks.Cache
	prod  *mocks.Producer
	svc   services.TaskService
}

func (s *TaskServiceSuite) SetupTest() {
	s.repo = mocks.NewTaskRepository(s.T())
	s.cache = mocks.NewCache(s.T())
	s.prod = mocks.NewProducer(s.T())
	s.svc = services.NewTaskService(s.repo, s.cache, s.prod, nil)
}

func (s *TaskServiceSuite) TestCreateTask_CallsRepoCacheAndPublishes() {
	ctx := context.Background()
	t := &models.Task{Title: "hello", OwnerID: 1}

	s.repo.On("Create", mock.Anything, t).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, t.OwnerID).Return(nil)
	s.prod.On("Publish", mock.Anything, "tasks.created", mock.Anything).Return(nil)

	err := s.svc.CreateTask(ctx, t)

	s.Require().NoError(err)
	s.repo.AssertExpectations(s.T())
	s.cache.AssertExpectations(s.T())
	s.prod.AssertExpectations(s.T())
}

func TestTaskServiceSuite(t *testing.T) {
	suite.Run(t, new(TaskServiceSuite))
}
