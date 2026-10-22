package dao

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/uch71109/tasks/model"
)

var ErrTaskNotFound = errors.New("task not found")

// ListOption selects a page of non-deleted tasks ordered by ID ascending.
type ListOption struct {
	Cursor uuid.UUID
	Limit  int
}

// UpdateOption holds the fields to change.
type UpdateOption struct {
	Name   *string
	Status *model.Status
}

type TaskDAO interface {
	Create(ctx context.Context, t model.Task) error
	Get(ctx context.Context, id uuid.UUID) (model.Task, error)
	Update(ctx context.Context, id uuid.UUID, opt UpdateOption) (model.Task, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opt ListOption) ([]model.Task, error)
}
