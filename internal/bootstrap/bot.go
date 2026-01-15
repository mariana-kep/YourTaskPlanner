package bootstrap

import (
	"fmt"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/mariana-kep/yourtaskplanner/internal/cache"
	"github.com/mariana-kep/yourtaskplanner/internal/config"
	"github.com/mariana-kep/yourtaskplanner/internal/kafka"
	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	pgrepo "github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
)

func InitBot() (services.TaskService, pgrepo.TaskRepository, *tgbot.BotAPI, *config.Config, error) {
	_ = godotenv.Load()

	cfg := config.LoadConfigFromEnv()
	logg := logger.New(cfg)

	db, err := sqlx.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("db open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	taskRepo := pgrepo.NewTaskRepository(db)
	cacheImpl := cache.NewRedisCache(rdb)
	kafkaProd := kafka.NewProducer(cfg.KafkaBroker)
	taskSvc := services.NewTaskService(taskRepo, cacheImpl, kafkaProd, logg)

	bot, err := tgbot.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("telegram bot create: %w", err)
	}
	bot.Debug = false

	return taskSvc, taskRepo, bot, cfg, nil
}
