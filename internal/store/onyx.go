// internal/store/onyx.go
package store

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	gen "github.com/OnyxDevTools/api-starter/internal/gen/onyx"
	"github.com/OnyxDevTools/onyx-database-go/contract"
	sdk "github.com/OnyxDevTools/onyx-database-go/onyx"
)

type Onyx struct {
	core sdk.Client
	db   gen.DB
}

func NewOnyx(ctx context.Context, cfg sdk.Config) (*Onyx, error) {
	cfg.WireFormat = sdk.WireFormatJSON
	cfg.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	core, err := sdk.Init(ctx, cfg)
	if err != nil {
		return nil, errors.New("Onyx configuration unavailable: set ONYX_CONFIG_PATH to a downloaded config or set ONYX_DATABASE_ID, ONYX_DATABASE_API_KEY and ONYX_DATABASE_API_SECRET")
	}
	return &Onyx{core: core, db: gen.Wrap(core)}, nil
}
func classify(err error) error {
	if err == nil {
		return nil
	}
	var e *contract.Error
	if errors.As(err, &e) {
		switch e.Meta["status"] {
		case http.StatusConflict:
			return errors.Join(ErrConflict, err)
		case http.StatusUnauthorized, http.StatusForbidden:
			return errors.Join(ErrAuth, err)
		case http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusInternalServerError:
			return errors.Join(ErrUnavailable, err)
		}
	}
	var network net.Error
	if errors.As(err, &network) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return errors.Join(ErrUnavailable, err)
	}
	return errors.Join(ErrUnexpected, err)
}
func pageError(err error, cursor string) error {
	var e *contract.Error
	if cursor != "" && errors.As(err, &e) && e.Meta["status"] == http.StatusBadRequest {
		return errors.Join(ErrCursor, err)
	}
	return classify(err)
}
func (r *Onyx) Ready(ctx context.Context) error {
	_, err := r.db.Tasks().Limit(1).Page(ctx, "")
	return classify(err)
}

func fromTask(v gen.Task) Task {
	return Task{ID: v.Id, Title: v.Title, Body: v.Body, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Completed: v.Completed}
}
func (r *Onyx) CreateTask(ctx context.Context, v Task) (Task, error) {
	_, err := r.core.Insert(ctx, "Task", v, nil)
	return v, classify(err)
}
func (r *Onyx) GetTask(ctx context.Context, id string) (Task, error) {
	v, err := r.db.Tasks().FindByID(ctx, id)
	if err != nil {
		return Task{}, classify(err)
	}
	if v.Id == "" {
		return Task{}, ErrNotFound
	}
	return fromTask(v), nil
}
func (r *Onyx) ListTasks(ctx context.Context, limit int, cursor string) (Page, error) {
	page, err := r.db.Tasks().OrderBy("id", true).Limit(limit).Page(ctx, cursor)
	if err != nil {
		return Page{}, pageError(err, cursor)
	}
	items := make([]Task, 0, len(page.Items))
	for _, v := range page.Items {
		items = append(items, fromTask(v))
	}
	return Page{Items: items, NextCursor: page.NextCursor}, nil
}
func (r *Onyx) UpdateTask(ctx context.Context, id, title, body string, completed bool, now time.Time) (Task, error) {
	updates := gen.NewTaskUpdates().SetTitle(title).SetBody(body).SetUpdatedAt(now).SetCompleted(completed)
	count, err := r.db.Tasks().Where(sdk.Eq("id", id)).SetTaskUpdates(updates).Update(ctx)
	if err != nil {
		return Task{}, classify(err)
	}
	if count == 0 {
		return Task{}, ErrNotFound
	}
	return r.GetTask(ctx, id)
}
func (r *Onyx) DeleteTask(ctx context.Context, id string) error {
	count, err := r.db.Tasks().Where(sdk.Eq("id", id)).Delete(ctx)
	if err != nil {
		return classify(err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}
