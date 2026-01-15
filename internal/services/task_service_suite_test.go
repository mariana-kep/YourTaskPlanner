package services_test

import (
	"context"
	"errors"
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
	t := &models.Task{ID: 1, Title: "hello", OwnerID: 1}

	s.repo.On("Create", mock.Anything, t).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, t.OwnerID).Return(nil)
	s.prod.On("Publish", mock.Anything, "tasks.created", mock.Anything).Return(nil)

	err := s.svc.CreateTask(ctx, t)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Create", mock.Anything, t)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, t.OwnerID)
	s.prod.AssertCalled(s.T(), "Publish", mock.Anything, "tasks.created", mock.Anything)
}

func (s *TaskServiceSuite) TestCreateTask_NoCacheNoProducer_DoesNotPanic() {
	// create svc with nil cache and nil producer
	svc := services.NewTaskService(s.repo, nil, nil, nil)
	ctx := context.Background()
	t := &models.Task{ID: 2, Title: "no-cache", OwnerID: 2}

	s.repo.On("Create", mock.Anything, t).Return(nil)

	err := svc.CreateTask(ctx, t)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Create", mock.Anything, t)
}

func (s *TaskServiceSuite) TestGetTasks_CacheHit() {
	ctx := context.Background()
	userID := int64(10)
	want := []models.Task{{ID: 11, Title: "from-cache", OwnerID: userID}}

	s.cache.On("GetTasks", mock.Anything, userID).Return(want, nil)

	tasks, err := s.svc.GetTasks(ctx, userID)

	s.NoError(err)
	s.Equal(want, tasks)
	s.repo.AssertNotCalled(s.T(), "GetByUser", mock.Anything, mock.Anything)
}

func (s *TaskServiceSuite) TestGetTasks_CacheMiss_UsesRepoAndSetsCache() {
	ctx := context.Background()
	userID := int64(20)
	want := []models.Task{{ID: 21, Title: "from-repo", OwnerID: userID}}

	s.cache.On("GetTasks", mock.Anything, userID).Return(nil, errors.New("cache miss"))
	s.repo.On("GetByUser", mock.Anything, userID).Return(want, nil)
	s.cache.On("SetTasks", mock.Anything, userID, want, mock.Anything).Return(nil)

	tasks, err := s.svc.GetTasks(ctx, userID)

	s.NoError(err)
	s.Equal(want, tasks)
	s.repo.AssertCalled(s.T(), "GetByUser", mock.Anything, userID)
	s.cache.AssertCalled(s.T(), "SetTasks", mock.Anything, userID, want, mock.Anything)
}

func (s *TaskServiceSuite) TestUpdateTask_OldOwnerExists_InvalidateBoth() {
	ctx := context.Background()
	t := &models.Task{ID: 100, OwnerID: 200}
	old := &models.Task{ID: 100, OwnerID: 111}

	s.repo.On("GetByID", mock.Anything, t.ID).Return(old, nil)
	s.repo.On("Update", mock.Anything, t).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, old.OwnerID).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, t.OwnerID).Return(nil)

	err := s.svc.UpdateTask(ctx, t)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "GetByID", mock.Anything, t.ID)
	s.repo.AssertCalled(s.T(), "Update", mock.Anything, t)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, old.OwnerID)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, t.OwnerID)
}

func (s *TaskServiceSuite) TestUpdateTask_NoExisting_InvalidateNewOwnerOnly() {
	ctx := context.Background()
	t := &models.Task{ID: 101, OwnerID: 202}

	s.repo.On("GetByID", mock.Anything, t.ID).Return(nil, errors.New("not found"))
	s.repo.On("Update", mock.Anything, t).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, t.OwnerID).Return(nil)

	err := s.svc.UpdateTask(ctx, t)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Update", mock.Anything, t)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, t.OwnerID)
}

func (s *TaskServiceSuite) TestDeleteTask_GetByIDError() {
	ctx := context.Background()
	id := int64(999)

	s.repo.On("GetByID", mock.Anything, id).Return(nil, errors.New("db error"))

	err := s.svc.DeleteTask(ctx, id)

	s.Error(err)
	s.repo.AssertNotCalled(s.T(), "Delete", mock.Anything, mock.Anything)
}

func (s *TaskServiceSuite) TestDeleteTask_Success() {
	ctx := context.Background()
	id := int64(300)
	task := &models.Task{ID: id, OwnerID: 400}

	s.repo.On("GetByID", mock.Anything, id).Return(task, nil)
	s.repo.On("Delete", mock.Anything, id).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, task.OwnerID).Return(nil)

	err := s.svc.DeleteTask(ctx, id)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Delete", mock.Anything, id)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, task.OwnerID)
}

func (s *TaskServiceSuite) TestAssignTask_GetByIDError() {
	ctx := context.Background()
	taskID := int64(555)
	userID := int64(666)

	s.repo.On("GetByID", mock.Anything, taskID).Return(nil, errors.New("not found"))

	err := s.svc.AssignTask(ctx, taskID, userID)

	s.Error(err)
	s.repo.AssertNotCalled(s.T(), "Assign", mock.Anything, mock.Anything, mock.Anything)
}

func (s *TaskServiceSuite) TestAssignTask_Success() {
	ctx := context.Background()
	taskID := int64(700)
	userID := int64(800)
	task := &models.Task{ID: taskID, OwnerID: 900}

	s.repo.On("GetByID", mock.Anything, taskID).Return(task, nil)
	s.repo.On("Assign", mock.Anything, taskID, userID).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, task.OwnerID).Return(nil)
	s.cache.On("InvalidateTasks", mock.Anything, userID).Return(nil)
	s.prod.On("Publish", mock.Anything, "tasks.assigned", mock.Anything).Return(nil)

	err := s.svc.AssignTask(ctx, taskID, userID)

	s.NoError(err)
	s.repo.AssertCalled(s.T(), "Assign", mock.Anything, taskID, userID)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, task.OwnerID)
	s.cache.AssertCalled(s.T(), "InvalidateTasks", mock.Anything, userID)
	s.prod.AssertCalled(s.T(), "Publish", mock.Anything, "tasks.assigned", mock.Anything)
}

func (s *TaskServiceSuite) TestGetShared_ProxiesToRepo() {
	ctx := context.Background()
	want := []models.Task{{ID: 1, Title: "shared"}}

	s.repo.On("GetShared", mock.Anything).Return(want, nil)

	got, err := s.svc.GetShared(ctx)

	s.NoError(err)
	s.Equal(want, got)
	s.repo.AssertCalled(s.T(), "GetShared", mock.Anything)
}

func (s *TaskServiceSuite) TestCreateTask_RepoError() {
	ctx := context.Background()
	t := &models.Task{ID: 3, Title: "fail", OwnerID: 3}

	s.repo.On("Create", mock.Anything, t).Return(errors.New("create failed"))

	err := s.svc.CreateTask(ctx, t)

	s.Error(err)
}

func (s *TaskServiceSuite) TestGetTasks_RepoErrorWhenCacheMiss() {
	ctx := context.Background()
	userID := int64(31)

	s.cache.On("GetTasks", mock.Anything, userID).Return(nil, errors.New("cache miss"))
	s.repo.On("GetByUser", mock.Anything, userID).Return(nil, errors.New("db err"))

	_, err := s.svc.GetTasks(ctx, userID)
	s.Error(err)
}

func (s *TaskServiceSuite) TestGetTasks_CacheEmptySlice_UsesRepo() {
	ctx := context.Background()
	userID := int64(41)
	s.cache.On("GetTasks", mock.Anything, userID).Return([]models.Task{}, nil)
	want := []models.Task{{ID: 42, Title: "from-repo", OwnerID: userID}}
	s.repo.On("GetByUser", mock.Anything, userID).Return(want, nil)
	s.cache.On("SetTasks", mock.Anything, userID, want, mock.Anything).Return(nil)

	tasks, err := s.svc.GetTasks(ctx, userID)
	s.NoError(err)
	s.Equal(want, tasks)
}

func (s *TaskServiceSuite) TestUpdateTask_UpdateError() {
	ctx := context.Background()
	t := &models.Task{ID: 201, OwnerID: 301}
	old := &models.Task{ID: 201, OwnerID: 401}

	s.repo.On("GetByID", mock.Anything, t.ID).Return(old, nil)
	s.repo.On("Update", mock.Anything, t).Return(errors.New("update failed"))

	err := s.svc.UpdateTask(ctx, t)
	s.Error(err)
}

func (s *TaskServiceSuite) TestDeleteTask_DeleteError() {
	ctx := context.Background()
	id := int64(303)
	task := &models.Task{ID: id, OwnerID: 404}

	s.repo.On("GetByID", mock.Anything, id).Return(task, nil)
	s.repo.On("Delete", mock.Anything, id).Return(errors.New("delete failed"))

	err := s.svc.DeleteTask(ctx, id)
	s.Error(err)
}

func (s *TaskServiceSuite) TestAssignTask_AssignError() {
	ctx := context.Background()
	taskID := int64(707)
	userID := int64(808)
	task := &models.Task{ID: taskID, OwnerID: 909}

	s.repo.On("GetByID", mock.Anything, taskID).Return(task, nil)
	s.repo.On("Assign", mock.Anything, taskID, userID).Return(errors.New("assign failed"))

	err := s.svc.AssignTask(ctx, taskID, userID)
	s.Error(err)
}

func TestTaskServiceSuite(t *testing.T) {
	suite.Run(t, new(TaskServiceSuite))
}
