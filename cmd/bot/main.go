package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/mariana-kep/yourtaskplanner/internal/cache"
	"github.com/mariana-kep/yourtaskplanner/internal/config"
	"github.com/mariana-kep/yourtaskplanner/internal/kafka"
	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	pgrepo "github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
	"github.com/mariana-kep/yourtaskplanner/internal/server"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
	"github.com/mariana-kep/yourtaskplanner/internal/worker"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()
	cfg := config.LoadConfigFromEnv()
	logg := logger.New(cfg)

	db, err := sqlx.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("db open: %v", err)
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
		log.Fatalf("telegram bot create: %v", err)
	}
	bot.Debug = false

	go kafka.ConsumeNotifications(cfg.KafkaBroker, "notifications.telegram", bot)

	go kafka.ConsumeRemindersAndPublishNotifications(cfg.KafkaBroker, kafkaProd)

	go worker.StartScheduler(taskRepo, kafkaProd, logg, time.Second*10)

	srv := server.New(cfg, taskSvc, logg, bot)
	addr := fmt.Sprintf(":%s", cfg.AppPort)
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Router(),
	}
	logg.Info("starting server", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	_ = context.Background()
}
