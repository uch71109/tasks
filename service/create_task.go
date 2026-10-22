package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uch71109/tasks/model"
)

type createTaskBody struct {
	Name   string        `json:"name" binding:"required,max=255"`
	Status *model.Status `json:"status" binding:"required"`
}

func (s *Service) CreateTask(c *gin.Context) {
	var body createTaskBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, err)
		return
	}
	name, err := normalizeName(body.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := validateStatus(*body.Status); err != nil {
		writeError(c, err)
		return
	}

	// UUIDv7 keeps IDs sorted by creation time, which pagination relies on.
	id, err := uuid.NewV7()
	if err != nil {
		writeError(c, fmt.Errorf("generate task id: %w", err))
		return
	}
	now := time.Now().UTC()
	task := model.Task{
		ID:        id,
		Name:      name,
		Status:    *body.Status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.TaskDAO.Create(c.Request.Context(), task); err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, task)
}
