package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Service) DeleteTask(c *gin.Context) {
	id, err := parseTaskID(c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}

	if err := s.TaskDAO.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
