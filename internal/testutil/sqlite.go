package testutil

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "modernc.org/sqlite"

	"github.com/uch71109/tasks/migrations"
)

// NewSQLiteDB returns a migrated in-memory SQLite database, set up like the
// one in main, that is closed when the current spec ends.
func NewSQLiteDB() *sql.DB {
	GinkgoHelper()
	dsn := "file:tasks-" + uuid.NewString() + "?mode=memory&cache=shared"

	// keeper holds an idle connection so the database outlives the pool's.
	keeper, err := sql.Open("sqlite", dsn)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(keeper.Close)
	Expect(keeper.Ping()).To(Succeed())

	db, err := sql.Open("sqlite", dsn)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(db.Close)
	db.SetMaxOpenConns(1)
	Expect(migrations.Up(context.Background(), db)).To(Succeed())
	return db
}
