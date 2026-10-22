package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/model"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type listTasksResponse struct {
	Items []model.Task `json:"items"`
	// NextKey is omitted on the last page.
	NextKey *string `json:"nextKey,omitempty"`
}

type nextKeyListTasks struct {
	Cursor uuid.UUID `json:"cursor"`
}

// ListTasks returns non-deleted tasks ordered by creation time, oldest first.
func (s *Service) ListTasks(c *gin.Context) {
	limit := DefaultPageSize
	if l := c.Query("limit"); l != "" {
		var err error
		if limit, err = strconv.Atoi(l); err != nil || limit < 1 || limit > MaxPageSize {
			writeError(c, &ValidationError{Field: "limit", Message: fmt.Sprintf("must be an integer between 1 and %d", MaxPageSize)})
			return
		}
	}

	nk, err := decodeNextKeyListTasks(c.Query("nextKey"))
	if err != nil {
		writeError(c, err)
		return
	}

	// Fetch one extra task to know whether there is a next page.
	tasks, err := s.TaskDAO.List(c.Request.Context(), dao.ListOption{Cursor: nk.Cursor, Limit: limit + 1})
	if err != nil {
		writeError(c, err)
		return
	}

	resp := listTasksResponse{Items: tasks}
	if len(tasks) > limit {
		resp.Items = tasks[:limit]
		nk.Cursor = resp.Items[limit-1].ID
		nextKey := encodeNextKeyListTasks(nk)
		resp.NextKey = &nextKey
	}
	c.JSON(http.StatusOK, resp)
}

func decodeNextKeyListTasks(nextKey string) (nextKeyListTasks, error) {
	if nextKey == "" {
		return nextKeyListTasks{}, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(nextKey)
	if err != nil {
		return nextKeyListTasks{}, ErrInvalidNextKey
	}

	var n nextKeyListTasks
	if err := json.Unmarshal(decoded, &n); err != nil || n.Cursor == uuid.Nil {
		return nextKeyListTasks{}, ErrInvalidNextKey
	}

	return n, nil
}

func encodeNextKeyListTasks(nextKey nextKeyListTasks) string {
	raw, _ := json.Marshal(nextKey)
	return base64.RawURLEncoding.EncodeToString(raw)
}
