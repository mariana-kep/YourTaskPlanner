package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/joho/godotenv"
	"github.com/mariana-kep/yourtaskplanner/internal/cache"
	"github.com/mariana-kep/yourtaskplanner/internal/config"
	tgHandler "github.com/mariana-kep/yourtaskplanner/internal/handlers/telegram"
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

	if cfg.TelegramToken != "" {
		commands := []map[string]string{
			{"command": "start", "description": "Перезапуск / старт"},
			{"command": "help", "description": "Что умеет бот / справка"},
			{"command": "newtask", "description": "Создать задачу"},
			{"command": "mytasks", "description": "Мои задачи"},
			{"command": "updatetask", "description": "Обновить задачу"},
			{"command": "deletetask", "description": "Удалить задачу"},
			{"command": "assign", "description": "Назначить задачу пользователю"},
			{"command": "alltasks", "description": "Показать общие задачи"},
			{"command": "myid", "description": "Показать ваш chat id"},
		}
		body := map[string]interface{}{"commands": commands}
		bb, _ := json.Marshal(body)
		_, _ = http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands", cfg.TelegramToken), "application/json", bytes.NewReader(bb))
	}

	if cfg.TelegramToken != "" {
		_, _ = http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", cfg.TelegramToken), "application/json", nil)
	}

	go kafka.ConsumeNotifications(cfg.KafkaBroker, "notifications.telegram", bot, taskRepo, true)
	go kafka.ConsumeRemindersAndPublishNotifications(cfg.KafkaBroker, kafkaProd)
	go worker.StartScheduler(taskRepo, kafkaProd, logg, time.Second*10)

	go startPolling(bot, taskSvc)

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

func startPolling(bot *tgbot.BotAPI, ts services.TaskService) {
	if bot == nil {
		return
	}
	handler := tgHandler.NewUpdateHandler(bot, ts)

	u := tgbot.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for upd := range updates {
		b, err := json.Marshal(upd)
		if err != nil {
			continue
		}
		req := httptest.NewRequest("POST", "/api/v1/bot/webhook", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
