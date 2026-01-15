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

	"github.com/mariana-kep/yourtaskplanner/internal/bootstrap"
	tgHandler "github.com/mariana-kep/yourtaskplanner/internal/handlers/telegram"
	"github.com/mariana-kep/yourtaskplanner/internal/kafka"
	"github.com/mariana-kep/yourtaskplanner/internal/server"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
	"github.com/mariana-kep/yourtaskplanner/internal/worker"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	taskSvc, taskRepo, bot, cfg, err := bootstrap.InitBot()
	if err != nil {
		log.Fatalf("init bot failed: %v", err)
	}

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
		_, _ = http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", cfg.TelegramToken), "application/json", nil)
	}

	go kafka.ConsumeNotifications(cfg.KafkaBroker, "notifications.telegram", bot, taskRepo, true)
	go kafka.ConsumeRemindersAndPublishNotifications(cfg.KafkaBroker, kafka.NewProducer(cfg.KafkaBroker))
	go worker.StartScheduler(taskRepo, kafka.NewProducer(cfg.KafkaBroker), nil, time.Second*10)

	go startPolling(bot, taskSvc)

	srv := server.New(cfg, taskSvc, nil, bot)
	addr := fmt.Sprintf(":%s", cfg.AppPort)
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Router(),
	}
	log.Printf("starting server, addr=%s", addr)
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
