package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/uch71109/tasks/model"
)

const taskColumns = `id, name, status, created_at, updated_at, deleted_at`

// SQLiteTaskDAO is a TaskDAO backed by SQLite.
type SQLiteTaskDAO struct {
	db *sql.DB
}

// NewSQLiteTaskDAO returns a TaskDAO using db, whose schema must already be
// applied with migrations.Up.
func NewSQLiteTaskDAO(db *sql.DB) *SQLiteTaskDAO {
	return &SQLiteTaskDAO{db: db}
}

func (d *SQLiteTaskDAO) Create(ctx context.Context, t model.Task) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO tasks (`+taskColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Status, t.CreatedAt, t.UpdatedAt, t.DeletedAt)
	return err
}

func (d *SQLiteTaskDAO) Get(ctx context.Context, id uuid.UUID) (model.Task, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+`
		 FROM tasks
		 WHERE id = ? AND deleted_at IS NULL`, id)

	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, ErrTaskNotFound
	}
	return t, err
}

func (d *SQLiteTaskDAO) Update(ctx context.Context, id uuid.UUID, opt UpdateOption) (model.Task, error) {
	row := d.db.QueryRowContext(ctx,
		`UPDATE tasks
		 SET name = COALESCE(?, name), status = COALESCE(?, status), updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL
		 RETURNING `+taskColumns,
		opt.Name, opt.Status, time.Now().UTC(), id)

	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, ErrTaskNotFound
	}
	return t, err
}

func (d *SQLiteTaskDAO) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	res, err := d.db.ExecContext(ctx,
		`UPDATE tasks
		 SET deleted_at = ?, updated_at = ?
		 WHERE id = ? AND deleted_at IS NULL`,
		now, now, id)
	return checkAffected(res, err)
}

func (d *SQLiteTaskDAO) List(ctx context.Context, opt ListOption) ([]model.Task, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT `+taskColumns+`
		 FROM tasks
		 WHERE deleted_at IS NULL AND id > ? ORDER BY id LIMIT ?`,
		opt.Cursor, opt.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.Task, 0, opt.Limit)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func scanTask(row interface{ Scan(...any) error }) (model.Task, error) {
	var t model.Task
	if err := row.Scan(&t.ID, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt); err != nil {
		return model.Task{}, err
	}

	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	if t.DeletedAt != nil {
		deletedAt := t.DeletedAt.UTC()
		t.DeletedAt = &deletedAt
	}
	return t, nil
}

func checkAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}
