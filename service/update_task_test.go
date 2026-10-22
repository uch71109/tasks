package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/model"
)

func ptr[T any](v T) *T { return &v }

var _ = Describe("UpdateTask", func() {
	var (
		id   uuid.UUID
		path string
	)

	BeforeEach(func() {
		id = uuid.Must(uuid.NewV7())
		path = "/tasks/" + id.String()
	})

	// expectUpdate asserts that body is passed to the DAO as want, and that the
	// task the DAO returns is the response.
	expectUpdate := func(body string, want dao.UpdateOption) {
		GinkgoHelper()
		updated := model.Task{ID: id, Name: gofakeit.Word(), Status: model.StatusCompleted}
		taskDAO.EXPECT().Update(gomock.Any(), id, want).Return(updated, nil)

		w := doRequest(http.MethodPut, path, body)
		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(decode[model.Task](w)).To(Equal(updated))
	}

	It("updates both fields, trimming the name", func() {
		name := gofakeit.Word()
		expectUpdate(`{"name":" `+name+` ","status":1}`, dao.UpdateOption{Name: &name, Status: ptr(model.StatusCompleted)})
	})

	It("updates only the name", func() {
		name := gofakeit.Word()
		expectUpdate(`{"name":"`+name+`"}`, dao.UpdateOption{Name: &name})
	})

	It("updates only the status", func() {
		expectUpdate(`{"status":1}`, dao.UpdateOption{Status: ptr(model.StatusCompleted)})
	})

	It("passes status 0 as a value rather than a missing field", func() {
		expectUpdate(`{"status":0}`, dao.UpdateOption{Status: ptr(model.StatusIncomplete)})
	})

	It("returns 404 when the task does not exist or is deleted", func() {
		taskDAO.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(model.Task{}, dao.ErrTaskNotFound)
		expectAPIError(doRequest(http.MethodPut, path, `{"status":1}`), http.StatusNotFound, CodeNotFound)
	})

	It("returns a generic 500 when the DAO fails", func() {
		boom := errors.New("dao fails")
		taskDAO.EXPECT().Update(gomock.Any(), id, gomock.Any()).Return(model.Task{}, boom)
		expectInternalError(doRequest(http.MethodPut, path, `{"status":1}`), boom)
	})
})

var _ = Describe("UpdateTask validation", func() {
	id := uuid.NewString()
	blankName := errorBody{Code: CodeValidationFailed, Message: "name: must not be empty", Field: "name"}
	badStatus := errorBody{Code: CodeValidationFailed, Message: "status: must be 0 (incomplete) or 1 (completed)", Field: "status"}
	noFields := errorBody{Code: CodeValidationFailed, Message: "at least one of name or status is required"}
	nameTooLong := errorBody{Code: CodeValidationFailed, Message: "name: must be at most 255 characters", Field: "name"}

	DescribeTable("rejects bad requests without touching the DAO",
		func(path, body string, status int, want errorBody) {
			w := doRequest(http.MethodPut, path, body)
			expectAPIError(w, status, want.Code)
			Expect(decode[errorResponse](w).Error).To(Equal(want))
		},
		Entry("invalid id", "/tasks/123", `{"name":"a","status":0}`, http.StatusBadRequest,
			errorBody{Code: CodeInvalidID, Message: "id must be a valid UUID"}),
		Entry("malformed json", "/tasks/"+id, `{`, http.StatusBadRequest,
			errorBody{Code: CodeInvalidJSON, Message: "malformed JSON"}),
		Entry("body over the size limit", "/tasks/"+id, `{"name":"`+strings.Repeat("a", maxBodyBytes)+`"}`, http.StatusRequestEntityTooLarge,
			errorBody{Code: CodeRequestTooLarge, Message: fmt.Sprintf("request body must be at most %d bytes", maxBodyBytes)}),
		Entry("blank name", "/tasks/"+id, `{"name":"  ","status":0}`, http.StatusUnprocessableEntity, blankName),
		Entry("name too long", "/tasks/"+id, `{"name":"`+strings.Repeat("a", 256)+`","status":0}`, http.StatusUnprocessableEntity, nameTooLong),
		Entry("name too long in multi-byte characters", "/tasks/"+id, `{"name":"`+strings.Repeat("任", 256)+`"}`, http.StatusUnprocessableEntity, nameTooLong),
		Entry("wrong status type", "/tasks/"+id, `{"status":"1"}`, http.StatusBadRequest,
			errorBody{Code: CodeInvalidJSON, Message: "status: has the wrong type", Field: "status"}),
		Entry("invalid status", "/tasks/"+id, `{"name":"a","status":5}`, http.StatusUnprocessableEntity, badStatus),
		Entry("only an invalid status", "/tasks/"+id, `{"status":5}`, http.StatusUnprocessableEntity, badStatus),
		Entry("only a blank name", "/tasks/"+id, `{"name":" "}`, http.StatusUnprocessableEntity, blankName),
		Entry("no fields", "/tasks/"+id, `{}`, http.StatusUnprocessableEntity, noFields),
		Entry("only an unknown field", "/tasks/"+id, `{"stauts":1}`, http.StatusUnprocessableEntity, noFields),
	)
})
