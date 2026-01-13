package postgres

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mariana-kep/yourtaskplanner/internal/models"
)

type TaskRepository interface {
	Create(ctx context.Context, t *models.Task) error
	GetByUser(ctx context.Context, userID int64) ([]models.Task, error)
	Update(ctx context.Context, t *models.Task) error
	Delete(ctx context.Context, id int64) error
	Assign(ctx context.Context, taskID, userID int64) error
	GetShared(ctx context.Context) ([]models.Task, error)
	GetDueReminders(ctx context.Context, before time.Time) ([]models.Task, error)
	MarkReminderScheduled(ctx context.Context, taskID int64) error
	MarkReminderSent(ctx context.Context, taskID int64) error
	GetByID(ctx context.Context, id int64) (*models.Task, error)
}

type taskRepository struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) Create(ctx context.Context, t *models.Task) error {
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	query := `INSERT INTO tasks (title, description, owner_id, assigned_to, shared, due_at, remind_at, reminder_scheduled, reminder_sent, created_at, updated_at)
	VALUES (:title, :description, :owner_id, :assigned_to, :shared, :due_at, :remind_at, :reminder_scheduled, :reminder_sent, :created_at, :updated_at) RETURNING id`
	rows, err := r.db.NamedQueryContext(ctx, query, t)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&t.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *taskRepository) GetByUser(ctx context.Context, userID int64) ([]models.Task, error) {
	var res []models.Task
	err := r.db.SelectContext(ctx, &res, `SELECT * FROM tasks WHERE owner_id=$1 OR assigned_to=$1 ORDER BY due_at NULLS LAST`, userID)
	return res, err
}

func (r *taskRepository) GetByID(ctx context.Context, id int64) (*models.Task, error) {
	var t models.Task
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tasks WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *taskRepository) Update(ctx context.Context, t *models.Task) error {
	t.UpdatedAt = time.Now()
	_, err := r.db.NamedExecContext(ctx, `UPDATE tasks SET title=:title, description=:description, assigned_to=:assigned_to, shared=:shared, due_at=:due_at, remind_at=:remind_at, updated_at=:updated_at WHERE id=:id`, t)
	return err
}

func (r *taskRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	return err
}

func (r *taskRepository) Assign(ctx context.Context, taskID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET assigned_to=$1, shared=true, updated_at=$2 WHERE id=$3`, userID, time.Now(), taskID)
	return err
}

func (r *taskRepository) GetShared(ctx context.Context) ([]models.Task, error) {
	var res []models.Task
	err := r.db.SelectContext(ctx, &res, `SELECT * FROM tasks WHERE shared=true ORDER BY due_at NULLS LAST`)
	return res, err
}

func (r *taskRepository) GetDueReminders(ctx context.Context, before time.Time) ([]models.Task, error) {
	var res []models.Task
	err := r.db.SelectContext(ctx, &res, `SELECT * FROM tasks WHERE remind_at IS NOT NULL AND remind_at <= $1 AND reminder_scheduled = false AND reminder_sent = false`, before)
	return res, err
}

func (r *taskRepository) MarkReminderScheduled(ctx context.Context, taskID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET reminder_scheduled = true, updated_at = $1 WHERE id = $2`, time.Now(), taskID)
	return err
}

func (r *taskRepository) MarkReminderSent(ctx context.Context, taskID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET reminder_sent = true, updated_at = $1 WHERE id = $2`, time.Now(), taskID)
	return err
}
