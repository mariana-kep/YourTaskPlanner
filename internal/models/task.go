package models

import "time"

type Task struct {
    ID          int64      `db:"id" json:"id"`
    Title       string     `db:"title" json:"title"`
    Description string     `db:"description" json:"description"`
    OwnerID     int64      `db:"owner_id" json:"owner_id"`
    AssignedTo  *int64     `db:"assigned_to" json:"assigned_to,omitempty"`
    Shared      bool       `db:"shared" json:"shared"`
    DueAt       *time.Time `db:"due_at" json:"due_at,omitempty"`
    RemindAt    *time.Time `db:"remind_at" json:"remind_at,omitempty"`
    CreatedAt   time.Time  `db:"created_at" json:"created_at"`
    UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}