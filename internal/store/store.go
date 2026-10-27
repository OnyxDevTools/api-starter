// internal/store/store.go
package store

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("record not found")
var ErrConflict = errors.New("record already exists")
var ErrUnavailable = errors.New("database unavailable")
var ErrAuth = errors.New("database authentication failed")
var ErrUnexpected = errors.New("unexpected storage response")
var ErrCursor = errors.New("invalid continuation cursor")

type Task struct {
	ID        string    `json:"id" format:"uuid"`
	Title     string    `json:"title" maxLength:"120"`
	Body      string    `json:"body" maxLength:"10000"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Page struct {
	Items      []Task `json:"items" nullable:"false"`
	NextCursor string `json:"nextCursor,omitempty"`
}
type Repository interface {
	CreateTask(context.Context, Task) (Task, error)
	GetTask(context.Context, string) (Task, error)
	ListTasks(context.Context, int, string) (Page, error)
	UpdateTask(context.Context, string, string, string, bool, time.Time) (Task, error)
	DeleteTask(context.Context, string) error
	Ready(context.Context) error
}
