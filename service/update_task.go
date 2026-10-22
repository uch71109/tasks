package service

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/model"
)

type updateTaskBody struct {
	Name   *string       `json:"name" binding:"omitempty,max=255"`
	Status *model.Status `json:"status"`
}

func (s *Service) UpdateTask(c *gin.Context) {
	id, err := parseTaskID(c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	var body updateTaskBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, err)
		return
	}
	if body.Name == nil && body.Status == nil {
		writeError(c, &ValidationError{Message: "at least one of name or status is required"})
		return
	}
	if body.Name != nil {
		name, err := normalizeName(*body.Name)
		if err != nil {
			writeError(c, err)
			return
		}
		body.Name = &name
	}
	if body.Status != nil {
		if err := validateStatus(*body.Status); err != nil {
			writeError(c, err)
			return
		}
	}

	task, err := s.TaskDAO.Update(c.Request.Context(), id, dao.UpdateOption{Name: body.Name, Status: body.Status})
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, task)
}
