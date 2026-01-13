package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func tryParseTime(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	s = strings.Map(func(r rune) rune {
		if r == '\u00A0' || r == '\u2007' || r == '\u202F' {
			return ' '
		}
		return r
	}, s)
	s = strings.ReplaceAll(s, "\u200B", "")

	s = strings.ReplaceAll(s, ",", " ")

	if parts := strings.Fields(s); len(parts) >= 2 {
		if len(parts[1]) > 0 {
			p := parts[1]
			if len(p) == 2 && strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
				parts[1] = parts[1] + ":00"
				s = strings.Join(parts, " ")
			}
			if strings.Contains(s, "T") {
				tidx := strings.Index(s, "T")
				after := s[tidx+1:]
				if len(after) == 2 || (len(after) >= 3 && after[2] == ' ') {
					if len(after) >= 2 {
						s = s[:tidx+1] + after[:2] + ":00"
					}
				}
			}
		}
	}

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02",
		"02.01.2006 15:04",
		"02.01.2006",
		"2006/01/02 15:04",
		"2006/01/02",
		"2006-1-2T15:04",
		"2006-1-2 15:04",
	}

	var parseErr error
	loc := time.Local
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, nil
		} else {
			parseErr = err
		}
	}
	if strings.Contains(s, " ") && !strings.Contains(s, "T") {
		try := strings.ReplaceAll(s, " ", "T")
		if t, err := time.ParseInLocation("2006-01-02T15:04", try, loc); err == nil {
			return t, nil
		} else {
			parseErr = err
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse time %q: %v", raw, parseErr)
}

func sendMessage(bot *tgbot.BotAPI, chatID int64, text string) {
	msg := tgbot.NewMessage(chatID, text)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("failed to send message to %d: %v", chatID, err)
	}
}

func main() {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is empty")
	}

	apiBase := os.Getenv("API_BASE")
	if apiBase == "" {
		apiBase = "http://localhost:8080"
	}

	bot, err := tgbot.NewBotAPI(token)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}
	bot.Debug = false
	log.Printf("authorized on account %s", bot.Self.UserName)

	u := tgbot.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	ctx := context.Background()

	for {
		select {
		case upd := <-updates:
			if upd.Message == nil {
				continue
			}
			chatID := upd.Message.Chat.ID
			text := strings.TrimSpace(upd.Message.Text)
			if text == "" {
				continue
			}

			log.Printf("update from %d: %s", chatID, text)

			switch {
			case text == "/start":
				sendMessage(bot, chatID, "Привет! Я YourTaskPlanner - твой бот-помощник по планированию задач. Используйте /newtask Название задачи | Описание | 2026-01-02T15:04 и /mytasks")

			case strings.HasPrefix(text, "/mytasks"):
				url := fmt.Sprintf("%s/api/v1/tasks?owner_id=%d", apiBase, chatID)
				resp, err := http.Get(url)
				if err != nil {
					log.Printf("error fetching tasks: %v", err)
					sendMessage(bot, chatID, "Ошибка при получении задач: "+err.Error())
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					log.Printf("tasks list returned status %d: %s", resp.StatusCode, string(body))
					sendMessage(bot, chatID, fmt.Sprintf("Ошибка: API вернул %d", resp.StatusCode))
					continue
				}

				var tasks []map[string]interface{}
				if err := json.Unmarshal(body, &tasks); err != nil {
					var wrapped map[string]interface{}
					if err2 := json.Unmarshal(body, &wrapped); err2 == nil {
						if d, ok := wrapped["data"]; ok {
							b, _ := json.Marshal(d)
							_ = json.Unmarshal(b, &tasks)
						}
					}
				}

				if len(tasks) == 0 {
					sendMessage(bot, chatID, "У вас пока нет задач.")
					continue
				}

				var sb strings.Builder
				sb.WriteString("Ваши задачи:\n")
				for _, t := range tasks {
					title := fmt.Sprintf("%v", t["title"])
					id := fmt.Sprintf("%v", t["id"])
					due := ""
					if v, ok := t["due_at"]; ok && v != nil {
						due = fmt.Sprintf(" (due: %v)", v)
					}
					sb.WriteString(fmt.Sprintf("- %s %s%s\n", id, title, due))
				}
				sendMessage(bot, chatID, sb.String())

			case strings.HasPrefix(text, "/newtask"):
				payload := strings.TrimSpace(strings.TrimPrefix(text, "/newtask"))
				if payload == "" {
					sendMessage(bot, chatID, "Формат: /newtask Название задачи | Описание | 2026-01-02T15:04")
					continue
				}
				parts := strings.Split(payload, "|")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				if len(parts) < 3 {
					sendMessage(bot, chatID, "Неверный формат. Используйте: /newtask Название задачи | Описание | 2026-01-02T15:04")
					continue
				}
				title := parts[0]
				desc := parts[1]
				dueStr := parts[2]

				log.Printf("parsing due string: %q", dueStr)
				due, err := tryParseTime(dueStr)
				if err != nil {
					log.Printf("parse error: %v", err)
					sendMessage(bot, chatID, "Не удалось распознать дату. Используйте формат 2026-01-02T15:04 или RFC3339. Пример: 2026-01-12T22:00 или 2026-01-12 22:00")
					continue
				}

				task := map[string]interface{}{
					"title":       title,
					"description": desc,
					"owner_id":    chatID,
					"due_at":      due.Format(time.RFC3339),
				}
				bts, _ := json.Marshal(task)
				url := fmt.Sprintf("%s/api/v1/tasks", apiBase)
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bts))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					log.Printf("error posting task to api: %v", err)
					sendMessage(bot, chatID, "Ошибка при создании задачи: "+err.Error())
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
					var respObj map[string]interface{}
					_ = json.Unmarshal(body, &respObj)
					var idStr string
					if r, ok := respObj["id"]; ok {
						idStr = fmt.Sprintf("%v", r)
					} else if data, ok := respObj["data"].(map[string]interface{}); ok {
						if r2, ok2 := data["id"]; ok2 {
							idStr = fmt.Sprintf("%v", r2)
						}
					}
					if idStr != "" {
						sendMessage(bot, chatID, "Задача создана, id: "+idStr)
					} else {
						sendMessage(bot, chatID, "Задача создана.")
					}
					log.Printf("task created for %d, resp: %s", chatID, string(body))
				} else {
					log.Printf("API returned %d: %s", resp.StatusCode, string(body))
					sendMessage(bot, chatID, fmt.Sprintf("Ошибка API: статус %d", resp.StatusCode))
				}

			default:
				sendMessage(bot, chatID, "Принял: "+text)
			}

		case <-time.After(time.Second * 1):
		}
	}
}
