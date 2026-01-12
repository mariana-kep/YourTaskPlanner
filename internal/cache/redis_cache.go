package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/redis/go-redis/v9"
)

type Cache interface {
	GetTasks(ctx context.Context, userID int64) ([]models.Task, error)
	SetTasks(ctx context.Context, userID int64, tasks []models.Task, ttl time.Duration) error
	InvalidateTasks(ctx context.Context, userID int64) error
}

type redisCache struct {
	rdb *redis.Client
}

func NewRedisCache(rdb *redis.Client) Cache {
	return &redisCache{rdb: rdb}
}

func (c *redisCache) cacheKey(userID int64) string {
	return "tasks:user:" + strconv.FormatInt(userID, 10)
}

func (c *redisCache) GetTasks(ctx context.Context, userID int64) ([]models.Task, error) {
	key := c.cacheKey(userID)
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var tasks []models.Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (c *redisCache) SetTasks(ctx context.Context, userID int64, tasks []models.Task, ttl time.Duration) error {
	key := c.cacheKey(userID)
	b, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

func (c *redisCache) InvalidateTasks(ctx context.Context, userID int64) error {
	key := c.cacheKey(userID)
	return c.rdb.Del(ctx, key).Err()
}
