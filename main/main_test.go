package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/internal/testutil"
)

func TestServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("serverArgs", func() {
	It("has defaults", func() {
		// Empty values count as unset, so the shell's environment can't leak in.
		GinkgoT().Setenv("HTTP_ADDR", "")
		GinkgoT().Setenv("SHUTDOWN_TIMEOUT", "")
		var args serverArgs
		Expect(env.Parse(&args)).To(Succeed())
		Expect(args).To(Equal(serverArgs{HTTPAddr: ":8080", ShutdownTimeout: 10 * time.Second}))
	})

	It("reads environment variables", func() {
		GinkgoT().Setenv("HTTP_ADDR", ":9090")
		GinkgoT().Setenv("SHUTDOWN_TIMEOUT", "3s")
		var args serverArgs
		Expect(env.Parse(&args)).To(Succeed())
		Expect(args).To(Equal(serverArgs{HTTPAddr: ":9090", ShutdownTimeout: 3 * time.Second}))
	})
})

var _ = Describe("run", func() {
	It("returns an error when the address is unavailable", func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(ln.Close)

		err = run(context.Background(), serverArgs{HTTPAddr: ln.Addr().String(), ShutdownTimeout: time.Second})
		Expect(err).To(HaveOccurred())
	})

	It("returns nil when cancelled during startup", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(run(ctx, serverArgs{HTTPAddr: "127.0.0.1:0", ShutdownTimeout: time.Second})).To(Succeed())
	})

	It("serves until the context is cancelled, then shuts down cleanly", func() {
		// Reserve a free port for run to listen on.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		addr := ln.Addr().String()
		Expect(ln.Close()).To(Succeed())

		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		done := make(chan error, 1)
		go func() {
			done <- run(ctx, serverArgs{HTTPAddr: addr, ShutdownTimeout: time.Second})
		}()

		Eventually(func() (int, error) {
			resp, err := http.Get("http://" + addr + "/healthz")
			if err != nil {
				return 0, err
			}
			resp.Body.Close()
			return resp.StatusCode, nil
		}).Should(Equal(http.StatusOK))

		cancel()
		Eventually(done).Should(Receive(BeNil()))
	})
})

var _ = Describe("openDB", func() {
	It("keeps the data when the pool closes its connection", func() {
		db, keeper, err := openDB(context.Background())
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(keeper.Close)
		DeferCleanup(db.Close)
		// Close the pool connection after every use, as database/sql does with
		// a connection it considers broken.
		db.SetMaxIdleConns(0)

		_, err = db.Exec(`INSERT INTO tasks (id, name, status, created_at, updated_at) VALUES ('a', 'a', 0, '', '')`)
		Expect(err).NotTo(HaveOccurred())
		var n int
		Expect(db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&n)).To(Succeed())
		Expect(n).To(Equal(1))
	})
})

var _ = Describe("newRouter", func() {
	var srv *httptest.Server

	BeforeEach(func() {
		srv = httptest.NewServer(newRouter(dao.NewSQLiteTaskDAO(testutil.NewSQLiteDB())))
		DeferCleanup(srv.Close)
	})

	It("serves the health check", func() {
		resp, err := http.Get(srv.URL + "/healthz")
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("wires the task routes end to end", func() {
		resp, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"name":"a","status":0}`))
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	})
})
