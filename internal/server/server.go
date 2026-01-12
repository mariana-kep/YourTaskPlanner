package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/mariana-kep/yourtaskplanner/internal/config"
	tgHandler "github.com/mariana-kep/yourtaskplanner/internal/handlers/telegram"
	"github.com/mariana-kep/yourtaskplanner/internal/logger"
	"github.com/mariana-kep/yourtaskplanner/internal/models"
	"github.com/mariana-kep/yourtaskplanner/internal/services"
)

type Server struct {
	cfg    *config.Config
	ts     services.TaskService
	logger *logger.Logger
	router *chi.Mux
	bot    *tgbot.BotAPI
}

func New(cfg *config.Config, ts services.TaskService, logger *logger.Logger, bot *tgbot.BotAPI) *Server {
	s := &Server{cfg: cfg, ts: ts, logger: logger, router: chi.NewRouter(), bot: bot}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := s.router
	r.Post("/api/v1/tasks", s.createTask)
	r.Get("/api/v1/tasks", s.listTasks)
	r.Put("/api/v1/tasks/{id}", s.updateTask)
	r.Delete("/api/v1/tasks/{id}", s.deleteTask)
	r.Post("/api/v1/tasks/{id}/assign", s.assignTask)
	r.Get("/api/v1/tasks/shared", s.sharedTasks)
	// telegram webhook
	r.Post("/api/v1/bot/webhook", tgHandler.NewUpdateHandler(s.bot, s.ts).ServeHTTP)
}

func (s *Server) Router() http.Handler {
	return s.router
}

type taskReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	OwnerID     int64  `json:"owner_id"`
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var tr taskReq
	_ = json.NewDecoder(r.Body).Decode(&tr)
	t := &models.Task{Title: tr.Title, Description: tr.Description, OwnerID: tr.OwnerID}
	_ = s.ts.CreateTask(r.Context(), t)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("user_id")
	uid, _ := strconv.ParseInt(q, 10, 64)
	tasks, _ := s.ts.GetTasks(r.Context(), uid)
	_ = json.NewEncoder(w).Encode(tasks)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var t models.Task
	_ = json.NewDecoder(r.Body).Decode(&t)
	t.ID = id
	_ = s.ts.UpdateTask(r.Context(), &t)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	_ = s.ts.DeleteTask(r.Context(), id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assignTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	var body struct {
		UserID int64 `json:"user_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = s.ts.AssignTask(r.Context(), id, body.UserID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) sharedTasks(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.ts.GetShared(r.Context())
	_ = json.NewEncoder(w).Encode(tasks)
}
