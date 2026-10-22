package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/migrations"
	"github.com/uch71109/tasks/service"
)

type serverArgs struct {
	HTTPAddr        string        `env:"HTTP_ADDR" envDefault:":8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

func main() {
	gin.SetMode(gin.ReleaseMode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var args serverArgs
	if err := env.Parse(&args); err != nil {
		fmt.Fprintln(os.Stderr, "parse args:", err)
		os.Exit(1)
	}
	if err := run(ctx, args); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args serverArgs) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, keeper, err := openDB(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil // a shutdown signal arrived during startup
		}
		return err
	}
	defer keeper.Close()
	defer db.Close()

	ln, err := net.Listen("tcp", args.HTTPAddr)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler: newRouter(dao.NewSQLiteTaskDAO(db)),
		// Close connections whose headers arrive too slowly (Slowloris).
		ReadHeaderTimeout: 5 * time.Second,
		// Log net/http's own errors as JSON too.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", ln.Addr().String())
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), args.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openDB(ctx context.Context) (db, keeper *sql.DB, err error) {
	// A named shared-cache database is shared by every connection that opens
	// the same name, and lives until its last connection closes.
	dsn := fmt.Sprintf("file:tasks-%s?mode=memory&cache=shared", uuid.NewString())

	// database/sql may close a pool connection at any time, e.g. after a query
	// is interrupted by a cancelled request. keeper holds an idle connection
	// that is never used, so the database outlives the pool's connections.
	keeper, err = sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := keeper.PingContext(ctx); err != nil {
		keeper.Close()
		return nil, nil, err
	}

	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		keeper.Close()
		return nil, nil, err
	}
	// One connection serializes access, so writers never hit shared-cache
	// table locks.
	db.SetMaxOpenConns(1)
	if err := migrations.Up(ctx, db); err != nil {
		db.Close()
		keeper.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	return db, keeper, nil
}

func newRouter(taskDAO dao.TaskDAO) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	svc := &service.Service{TaskDAO: taskDAO}
	svc.Register(r)
	return r
}
