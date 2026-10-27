// internal/httpapi/tasks.go
package httpapi

import (
	"context"
	"github.com/OnyxDevTools/api-starter/internal/store"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"log/slog"
	"time"
)

func registerTasks(api huma.API, repo store.Repository, log *slog.Logger) {
	path := "/api/v1/tasks"
	huma.Register(api, operation("create-task", "POST", path, "Tasks", "Create a task", 201), func(ctx context.Context, input *TaskWrite) (*TaskCreated, error) {
		now := time.Now().UTC()
		v := store.Task{ID: uuid.NewString(), Title: input.Body.Title, Body: input.Body.Body, CreatedAt: now, UpdatedAt: now, Completed: input.Body.Completed}
		out, err := repo.CreateTask(ctx, v)
		if err != nil {
			return nil, failure(ctx, log, err)
		}
		return &TaskCreated{Location: path + "/" + out.ID, Body: out}, nil
	})
	huma.Register(api, operation("list-tasks", "GET", path, "Tasks", "List tasks by ID with cursor pagination", 200), func(ctx context.Context, input *ListInput) (*TasksOutput, error) {
		out, err := repo.ListTasks(ctx, input.Limit, input.Cursor)
		if err != nil {
			return nil, failure(ctx, log, err)
		}
		return &TasksOutput{Body: out}, nil
	})
	huma.Register(api, operation("get-task", "GET", path+"/{id}", "Tasks", "Get a task", 200), func(ctx context.Context, input *IDInput) (*TaskOutput, error) {
		out, err := repo.GetTask(ctx, input.ID)
		if err != nil {
			return nil, failure(ctx, log, err)
		}
		return &TaskOutput{Body: out}, nil
	})
	huma.Register(api, operation("update-task", "PUT", path+"/{id}", "Tasks", "Replace a task", 200), func(ctx context.Context, input *TaskUpdate) (*TaskOutput, error) {
		out, err := repo.UpdateTask(ctx, input.ID, input.Body.Title, input.Body.Body, input.Body.Completed, time.Now().UTC())
		if err != nil {
			return nil, failure(ctx, log, err)
		}
		return &TaskOutput{Body: out}, nil
	})
	huma.Register(api, operation("delete-task", "DELETE", path+"/{id}", "Tasks", "Delete a task", 204), func(ctx context.Context, input *IDInput) (*struct{}, error) {
		if err := repo.DeleteTask(ctx, input.ID); err != nil {
			return nil, failure(ctx, log, err)
		}
		return nil, nil
	})
}
