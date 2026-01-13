package server

import (
	"database/sql"
	"encoding/json"
	"errors"
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

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var tr taskReq
	if err := json.NewDecoder(r.Body).Decode(&tr); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if tr.Title == "" || tr.OwnerID == 0 {
		http.Error(w, "title and owner_id are required", http.StatusBadRequest)
		return
	}
	t := &models.Task{Title: tr.Title, Description: tr.Description, OwnerID: tr.OwnerID}
	if err := s.ts.CreateTask(r.Context(), t); err != nil {
		if s.logger != nil {
			s.logger.Error("createTask error", "error", err)
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("user_id")
	if q == "" {
		http.Error(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}
	uid, err := strconv.ParseInt(q, 10, 64)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	tasks, err := s.ts.GetTasks(r.Context(), uid)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("listTasks error", "error", err)
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var t models.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	t.ID = id
	if err := s.ts.UpdateTask(r.Context(), &t); err != nil {
		if s.logger != nil {
			s.logger.Error("updateTask error", "error", err)
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.ts.DeleteTask(r.Context(), id); err != nil {
		if s.logger != nil {
			s.logger.Error("deleteTask error", "error", err)
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) assignTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.UserID == 0 {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	if err := s.ts.AssignTask(r.Context(), id, body.UserID); err != nil {
		if s.logger != nil {
			s.logger.Error("assignTask error", "error", err)
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) sharedTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.ts.GetShared(r.Context())
	if err != nil {
		if s.logger != nil {
			s.logger.Error("sharedTasks error", "error", err)
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}
