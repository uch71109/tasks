package service

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uch71109/tasks/dao"
)

// Service implements the task API.
type Service struct {
	TaskDAO dao.TaskDAO
}

// maxBodyBytes caps request bodies 1 MiB.
const maxBodyBytes = 1 << 20

// Register mounts the task routes on r.
func (s *Service) Register(r gin.IRouter) {
	tasks := r.Group("/tasks", limitBody)
	tasks.GET("", s.ListTasks)
	tasks.POST("", s.CreateTask)
	tasks.PUT("/:id", s.UpdateTask)
	tasks.DELETE("/:id", s.DeleteTask)
}

func limitBody(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	c.Next()
}
