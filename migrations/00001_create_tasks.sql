-- +goose Up
CREATE TABLE tasks (
  id         TEXT PRIMARY KEY, -- UUIDv7 text sorts by creation time
  name       TEXT NOT NULL,
  status     INTEGER NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME
);

-- +goose Down
DROP TABLE tasks;
