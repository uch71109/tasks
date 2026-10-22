package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/model"
)

var (
	// ErrInvalidTaskID is returned when a task ID is not a valid UUID.
	ErrInvalidTaskID = errors.New("invalid task id")
	// ErrInvalidNextKey is returned when a pagination key cannot be decoded.
	ErrInvalidNextKey = errors.New("invalid nextKey")
)

// ValidationError describes an invalid input field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

// ErrorCode identifies the kind of error in an error response.
type ErrorCode string

const (
	CodeInvalidJSON      ErrorCode = "invalid_json"
	CodeInvalidID        ErrorCode = "invalid_id"
	CodeInvalidNextKey   ErrorCode = "invalid_next_key"
	CodeNotFound         ErrorCode = "not_found"
	CodeRequestTooLarge  ErrorCode = "request_too_large"
	CodeValidationFailed ErrorCode = "validation_failed"
	CodeInternalError    ErrorCode = "internal_error"
)

type errorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

// writeError maps an error, from the service or from binding a request body,
// to its HTTP status and the API's error format.
func writeError(c *gin.Context, err error) {
	// A failed binding tag is reported like the service's own validation.
	var fieldErrs validator.ValidationErrors
	if errors.As(err, &fieldErrs) {
		fe := fieldErrs[0]
		message := "is invalid"
		switch fe.Tag() {
		case "required":
			message = "is required"
		case "max":
			// validator counts characters (runes), not bytes.
			message = fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		err = &ValidationError{Field: strings.ToLower(fe.Field()), Message: message}
	}

	var (
		valErr      *ValidationError
		maxBytesErr *http.MaxBytesError
		typeErr     *json.UnmarshalTypeError
		syntaxErr   *json.SyntaxError
	)
	switch {
	case errors.As(err, &valErr):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: errorBody{
			Code:    CodeValidationFailed,
			Message: valErr.Error(),
			Field:   valErr.Field,
		}})
	case errors.As(err, &maxBytesErr):
		c.JSON(http.StatusRequestEntityTooLarge, errorResponse{Error: errorBody{
			Code:    CodeRequestTooLarge,
			Message: fmt.Sprintf("request body must be at most %d bytes", maxBytesErr.Limit),
		}})
	case errors.As(err, &typeErr) && typeErr.Field != "":
		// Report only the field, not the decoder's message with Go type names.
		c.JSON(http.StatusBadRequest, errorResponse{Error: errorBody{
			Code:    CodeInvalidJSON,
			Message: typeErr.Field + ": has the wrong type",
			Field:   typeErr.Field,
		}})
	case errors.As(err, &typeErr), errors.As(err, &syntaxErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		// Not a JSON object, invalid JSON, an empty body, or a truncated one.
		c.JSON(http.StatusBadRequest, errorResponse{Error: errorBody{Code: CodeInvalidJSON, Message: "malformed JSON"}})
	case errors.Is(err, ErrInvalidTaskID):
		c.JSON(http.StatusBadRequest, errorResponse{Error: errorBody{Code: CodeInvalidID, Message: "id must be a valid UUID"}})
	case errors.Is(err, dao.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: errorBody{Code: CodeNotFound, Message: "task not found"}})
	case errors.Is(err, ErrInvalidNextKey):
		c.JSON(http.StatusBadRequest, errorResponse{Error: errorBody{Code: CodeInvalidNextKey, Message: err.Error()}})
	default:
		slog.ErrorContext(c.Request.Context(), "internal error",
			"method", c.Request.Method, "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: errorBody{Code: CodeInternalError, Message: "internal server error"}})
	}
}

func parseTaskID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, ErrInvalidTaskID
	}
	return parsed, nil
}

// normalizeName trims surrounding whitespace and rejects a blank name. The
// length limit is declared on the request bodies with binding:"max=255".
func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &ValidationError{Field: "name", Message: "must not be empty"}
	}
	return name, nil
}

func validateStatus(s model.Status) error {
	if !s.Valid() {
		return &ValidationError{Field: "status", Message: "must be 0 (incomplete) or 1 (completed)"}
	}
	return nil
}
