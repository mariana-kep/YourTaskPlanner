package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx"
	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/mariana-kep/yourtaskplanner/internal/cache"
	"github.com/mariana-kep/yourtaskplanner/internal/config"
	"github.com/mariana-kep/yourtaskplanner/internal/kafka"
	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	"github.com/mariana-kep/yourtaskplanner/internal/repo/postgres"
	"github.com/mariana-kep/yourtaskplanner/internal/server"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
	"github.com/mariana-kep/yourtaskplanner/internal/worker"
)

func main() {
	cfg := config.LoadConfigFromEnv()
	logg := logger.New(cfg)
	db, err := sqlx.Connect("postgres", cfg.PostgresDSN())
	if err != nil {
		log.Fatal("db connect:", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	_ = rdb.Ping(context.Background()).Err()
	prod := kafka.NewProducer(cfg.KafkaBroker)
	taskRepo := postgres.NewTaskRepository(db)
	cacheClient := cache.NewRedisCache(rdb)
	taskSvc := services.NewTaskService(taskRepo, cacheClient, prod, logg)
	var botAPI *tgbot.BotAPI
	if cfg.TelegramToken != "" {
		b, err := tgbot.NewBotAPI(cfg.TelegramToken)
		if err == nil {
			botAPI = b
			_, _ = botAPI.Request(tgbot.NewRemoveWebhook())
		}
	}
	s := server.New(cfg, taskSvc, logg, botAPI)
	go worker.StartScheduler(taskRepo, prod, logg, time.Second*30)
	addr := fmt.Sprintf(":%s", cfg.AppPort)
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: s.Router(),
	}
	logg.Infof("starting http server on %s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}