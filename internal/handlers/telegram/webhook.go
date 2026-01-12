package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
)

type UpdateHandler struct {
	Bot *tgbot.BotAPI
	TS  services.TaskService
}

func NewUpdateHandler(bot *tgbot.BotAPI, ts services.TaskService) *UpdateHandler {
	return &UpdateHandler{Bot: bot, TS: ts}
}

func (h *UpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var upd tgbot.Update
	_ = json.NewDecoder(r.Body).Decode(&upd)
	if upd.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	chatID := upd.Message.Chat.ID
	text := strings.TrimSpace(upd.Message.Text)
	switch {
	case strings.HasPrefix(text, "/start"):
		msg := tgbot.NewMessage(chatID, "Привет! Я YourTaskPlanner. /newtask Title | Description | 2026-01-12T15:04 /mytasks")
		_, _ = h.Bot.Send(msg)
	case strings.HasPrefix(text, "/newtask"):
		payload := strings.TrimPrefix(text, "/newtask")
		parts := strings.Split(strings.TrimSpace(payload), "|")
		title := ""
		desc := ""
		var due *time.Time
		if len(parts) > 0 {
			title = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			desc = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[2])); err == nil {
				due = &t
			} else if t2, err2 := time.Parse("2006-01-02T15:04", strings.TrimSpace(parts[2])); err2 == nil {
				due = &t2
			}
		}
		task := &models.Task{
			Title:       title,
			Description: desc,
			OwnerID:     chatID,
			DueAt:       due,
		}
		_ = h.TS.CreateTask(context.Background(), task)
		msg := tgbot.NewMessage(chatID, "Задача создана: "+title)
		_, _ = h.Bot.Send(msg)
	case strings.HasPrefix(text, "/mytasks"):
		tasks, err := h.TS.GetTasks(context.Background(), chatID)
		if err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "Ошибка при получении задач"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(tasks) == 0 {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "У вас пока нет задач"))
			w.WriteHeader(http.StatusOK)
			return
		}
		var b strings.Builder
		for i, t := range tasks {
			b.WriteString(strconv.Itoa(i+1) + ". " + t.Title)
			if t.DueAt != nil {
				b.WriteString(" (due: " + t.DueAt.Format(time.RFC3339) + ")")
			}
			b.WriteString("\n")
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, b.String()))
	default:
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "Команда не распознана. Используйте /newtask или /mytasks"))
	}
	w.WriteHeader(http.StatusOK)
}
