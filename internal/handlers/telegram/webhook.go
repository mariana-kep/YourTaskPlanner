package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
)

func parseDateTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02",
		"02.01.2006 15:04",
		"02.01.2006 15:4",
		"02.01.2006 2:04",
		"02.01.2006",
	}

	var parsed time.Time
	var err error
	for _, f := range formats {
		parsed, err = time.ParseInLocation(f, s, time.Local)
		if err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}

	return nil, fmt.Errorf("invalid datetime format: %q", s)
}

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
		msgText := "👋 Привет! Я YourTaskPlanner — твой бот-помощник для создания и управления задачами.\n\n" +
			"Напиши /help, чтобы получить список команд и примеры формата ввода."

		keyboard := tgbot.NewReplyKeyboard(
			tgbot.NewKeyboardButtonRow(
				tgbot.NewKeyboardButton("/newtask"),
				tgbot.NewKeyboardButton("/mytasks"),
			),
			tgbot.NewKeyboardButtonRow(
				tgbot.NewKeyboardButton("/updatetask"),
				tgbot.NewKeyboardButton("/deletetask"),
			),
			tgbot.NewKeyboardButtonRow(
				tgbot.NewKeyboardButton("/assign"),
				tgbot.NewKeyboardButton("/alltasks"),
			),
			tgbot.NewKeyboardButtonRow(
				tgbot.NewKeyboardButton("/help"),
				tgbot.NewKeyboardButton("/myid"),
			),
		)
		msg := tgbot.NewMessage(chatID, msgText)
		msg.ReplyMarkup = keyboard
		_, _ = h.Bot.Send(msg)

	case strings.HasPrefix(text, "/help"):
		help := "📚 Справка по командам (формат и примеры):\n\n" +
			"🆕 /newtask Title | Description | DUE\n" +
			"  • Создать задачу. Поля разделяются символом | (pipe).\n" +
			"  • Title — заголовок (обязательно).\n" +
			"  • Description — описание (необязательно).\n" +
			"  • DUE — дата/время в любом формате:\n" +
			"   - 2026-01-02T15:04:05\n" +
			"	- 2026-01-02T15:04\n" +
			"	- 2026-01-02 15:04:05\n" +
			"	- 2026-01-02 15:04\n" +
			"	- 2026-01-02 15:04:05\n" +
			"	- 2026-01-02\n" +
			"	- 02.01.2026 15:04\n" +
			"	- 02.01.2026\n" +
			"	- 2026/01/02 15:04\n" +
			"	- 2026/01/02\n" +
			"	- 2026-1-2T15:04\n" +
			"	- 2026-1-2 15:04\n" +
			"  Пример: /newtask Купить хлеб | В булошной | 2026-01-15T09:00\n\n" +
			"📝 /mytasks\n" +
			"  • Показать ваши задачи.\n\n" +
			"✏️ /updatetask ID | Title | Description | assigned_to | shared | due_at | remind_at\n" +
			"  • Обновить задачу по ID. Оставьте поле пустым, если не хотите менять его.\n" +
			"  • assigned_to — chat id пользователя (число) или пусто.\n" +
			"  • shared — true|false\n" +
			"  Пример: /updatetask 42 | Новая тема | Описание | 123456789 | true | 2026-02-01T12:00 | 2026-02-01T11:00\n\n" +
			"🗑️ /deletetask ID\n" +
			"  • Удалить задачу по ID. \nПример: /deletetask 42\n\n" +
			"👥 /assign ID | USER_ID\n" +
			"  • Назначить задачу пользователю по его chat id. \nПример: /assign 42 | 123456789\n\n" +
			"📂 /alltasks\n" +
			"  • Показать общие задачи.\n\n" +
			"🔎 /myid\n" +
			"  • Показать ваш Telegram chat id (нужно для назначения задач другим людям)."
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, help))

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
			if t, err := parseDateTime(strings.TrimSpace(parts[2])); err == nil {
				due = t
			}
		}
		if title == "" {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ У задачи должен быть заголовок. Формат: /newtask Title | Description | DUE"))
			w.WriteHeader(http.StatusOK)
			return
		}
		task := &models.Task{
			Title:       title,
			Description: desc,
			OwnerID:     chatID,
			DueAt:       due,
			RemindAt:    due,
		}
		if err := h.TS.CreateTask(context.Background(), task); err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при создании задачи"))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "✅ Задача создана: "+title+" (ID:"+strconv.FormatInt(task.ID, 10)+")"))

	case strings.HasPrefix(text, "/mytasks"):
		tasks, err := h.TS.GetTasks(context.Background(), chatID)
		if err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при получении задач"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(tasks) == 0 {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "📭 У вас пока нет задач"))
			w.WriteHeader(http.StatusOK)
			return
		}

		var b strings.Builder
		for i, t := range tasks {
			b.WriteString(strconv.Itoa(i+1) + ". ID:" + strconv.FormatInt(t.ID, 10) + " — " + t.Title)
			if t.RemindAt != nil {
				b.WriteString(" (remind: " + t.RemindAt.In(time.Local).Format("2006-01-02 15:04") + ")")
			} else if t.DueAt != nil {
				b.WriteString(" (due: " + t.DueAt.In(time.Local).Format("2006-01-02 15:04") + ")")
			}
			b.WriteString("\n")
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "🗒️ Ваши задачи:\n"+b.String()))

	case strings.HasPrefix(text, "/updatetask"):
		payload := strings.TrimPrefix(text, "/updatetask")
		parts := strings.Split(strings.TrimSpace(payload), "|")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Укажите ID задачи. Пример: /updatetask 42 | Title | Description | 12345 | true | 2026-02-01T12:00 | 2026-02-01T11:00"))
			w.WriteHeader(http.StatusOK)
			return
		}
		id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Неверный ID задачи"))
			w.WriteHeader(http.StatusOK)
			return
		}
		var title, desc string
		var assignedPtr *int64
		var shared bool
		var due, remind *time.Time

		if len(parts) > 1 {
			title = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			desc = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			as := strings.TrimSpace(parts[3])
			if as != "" {
				if v, err := strconv.ParseInt(as, 10, 64); err == nil {
					assignedPtr = &v
				}
			}
		}
		if len(parts) > 4 {
			sh := strings.TrimSpace(parts[4])
			if sh != "" {
				if v, err := strconv.ParseBool(sh); err == nil {
					shared = v
				}
			}
		}
		if len(parts) > 5 {
			if t, err := parseDateTime(strings.TrimSpace(parts[5])); err == nil {
				due = t
			}
		}
		if len(parts) > 6 {
			if t, err := parseDateTime(strings.TrimSpace(parts[6])); err == nil {
				remind = t
			}
		}

		t := &models.Task{
			ID:          id,
			Title:       title,
			Description: desc,
			AssignedTo:  assignedPtr,
			Shared:      shared,
			DueAt:       due,
			RemindAt:    remind,
		}
		if err := h.TS.UpdateTask(context.Background(), t); err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при обновлении задачи: "+err.Error()))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "✅ Задача обновлена (ID:"+strconv.FormatInt(id, 10)+")"))

	case strings.HasPrefix(text, "/deletetask"):
		payload := strings.TrimSpace(strings.TrimPrefix(text, "/deletetask"))
		if payload == "" {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Укажите ID задачи. Пример: /deletetask 42"))
			w.WriteHeader(http.StatusOK)
			return
		}
		id, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Неверный ID"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := h.TS.DeleteTask(context.Background(), id); err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при удалении задачи: "+err.Error()))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "🗑️ Задача удалена (ID:"+strconv.FormatInt(id, 10)+")"))

	case strings.HasPrefix(text, "/assign"):
		payload := strings.TrimPrefix(text, "/assign")
		parts := strings.Split(strings.TrimSpace(payload), "|")
		if len(parts) < 2 {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Формат: /assign ID | USER_ID"))
			w.WriteHeader(http.StatusOK)
			return
		}
		id, err1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		uid, err2 := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err1 != nil || err2 != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "⚠️ Неверный ID задачи или user_id"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := h.TS.AssignTask(context.Background(), id, uid); err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при назначении: "+err.Error()))
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "👥 Задача назначена (ID:"+strconv.FormatInt(id, 10)+")"))

	case strings.HasPrefix(text, "/alltasks") || strings.HasPrefix(text, "/shared"):
		tasks, err := h.TS.GetShared(context.Background())
		if err != nil {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❗ Ошибка при получении общих задач"))
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(tasks) == 0 {
			_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "📭 Общих задач пока нет"))
			w.WriteHeader(http.StatusOK)
			return
		}
		var b strings.Builder
		for i, t := range tasks {
			b.WriteString(strconv.Itoa(i+1) + ". ID:" + strconv.FormatInt(t.ID, 10) + " — " + t.Title)
			if t.DueAt != nil {
				b.WriteString(" (due: " + t.DueAt.Format(time.RFC3339) + ")")
			}
			b.WriteString("\n")
		}
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "📂 Общие задачи:\n"+b.String()))

	case strings.HasPrefix(text, "/myid"):
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "🆔 Ваш chat id: "+strconv.FormatInt(chatID, 10)))

	default:
		_, _ = h.Bot.Send(tgbot.NewMessage(chatID, "❓ Команда не распознана. Используйте /help для списка команд."))
	}

	w.WriteHeader(http.StatusOK)
}
