package service

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/uch71109/tasks/dao"
)

var _ = Describe("DeleteTask", func() {
	var (
		path string
		w    *httptest.ResponseRecorder
		id   uuid.UUID
	)

	JustBeforeEach(func() {
		w = doRequest(http.MethodDelete, path, "")
	})

	BeforeEach(func() {
		id = uuid.Must(uuid.NewV7())
		path = "/tasks/" + id.String()
	})

	Describe("normal case", func() {
		BeforeEach(func() {
			taskDAO.EXPECT().Delete(gomock.Any(), id).Return(nil)
		})

		It("soft-deletes the task", func() {
			Expect(w.Code).To(Equal(http.StatusNoContent))
			Expect(w.Body.Len()).To(BeZero())
		})
	})

	When("the ID is not a UUID", func() {
		BeforeEach(func() {
			path = "/tasks/nope"
		})

		It("returns 400", func() {
			expectAPIError(w, http.StatusBadRequest, CodeInvalidID)
		})
	})

	When("the task does not exist or is already deleted", func() {
		BeforeEach(func() {
			taskDAO.EXPECT().Delete(gomock.Any(), id).Return(dao.ErrTaskNotFound)
		})

		It("returns 404", func() {
			expectAPIError(w, http.StatusNotFound, CodeNotFound)
		})
	})

	When("the DAO fails", func() {
		err := errors.New("dao fails")

		BeforeEach(func() {
			taskDAO.EXPECT().Delete(gomock.Any(), id).Return(err)
		})

		It("returns a generic 500", func() {
			expectInternalError(w, err)
		})
	})
})
