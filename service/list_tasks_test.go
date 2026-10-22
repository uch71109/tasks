package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/uch71109/tasks/dao"
	"github.com/uch71109/tasks/model"
)

var _ = Describe("ListTasks", func() {
	var (
		path  string
		w     *httptest.ResponseRecorder
		tasks []model.Task
	)

	JustBeforeEach(func() {
		w = doRequest(http.MethodGet, path, "")
	})

	BeforeEach(func() {
		path = "/tasks?limit=2"
		tasks = make([]model.Task, 3)
		for i := range tasks {
			tasks[i] = model.Task{ID: uuid.Must(uuid.NewV7()), Name: gofakeit.Word()}
		}
	})

	When("there are more tasks than the limit", func() {
		BeforeEach(func() {
			// One extra task is fetched to detect the next page.
			taskDAO.EXPECT().List(gomock.Any(), dao.ListOption{Cursor: uuid.Nil, Limit: 3}).Return(tasks, nil)
		})

		It("returns one page and a next key that continues after its last task", func() {
			Expect(w.Code).To(Equal(http.StatusOK))
			resp := decode[listTasksResponse](w)
			Expect(resp.Items).To(Equal(tasks[:2]))
			Expect(resp.NextKey).NotTo(BeNil())

			// Send the returned key back as a client would.
			taskDAO.EXPECT().List(gomock.Any(), dao.ListOption{Cursor: tasks[1].ID, Limit: 3}).Return(tasks[2:], nil)
			next := doRequest(http.MethodGet, path+"&nextKey="+url.QueryEscape(*resp.NextKey), "")
			Expect(next.Code).To(Equal(http.StatusOK))
			resp = decode[listTasksResponse](next)
			Expect(resp.Items).To(Equal(tasks[2:]))
			Expect(resp.NextKey).To(BeNil())
		})
	})

	When("the tasks fit exactly in one page", func() {
		BeforeEach(func() {
			taskDAO.EXPECT().List(gomock.Any(), dao.ListOption{Cursor: uuid.Nil, Limit: 3}).Return(tasks[:2], nil)
		})

		It("returns no next key", func() {
			Expect(w.Code).To(Equal(http.StatusOK))
			resp := decode[listTasksResponse](w)
			Expect(resp.Items).To(Equal(tasks[:2]))
			Expect(resp.NextKey).To(BeNil())
		})
	})

	When("there are no tasks and no limit is given", func() {
		BeforeEach(func() {
			path = "/tasks"
			taskDAO.EXPECT().List(gomock.Any(), dao.ListOption{Cursor: uuid.Nil, Limit: DefaultPageSize + 1}).Return([]model.Task{}, nil)
		})

		It("uses the default page size, serializes an empty list as [] and omits nextKey", func() {
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(MatchJSON(`{"items":[]}`))
		})
	})

	When("the DAO fails", func() {
		err := errors.New("dao fails")

		BeforeEach(func() {
			taskDAO.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, err)
		})

		It("returns a generic 500", func() {
			expectInternalError(w, err)
		})
	})
})

var _ = Describe("ListTasks limit", func() {
	DescribeTable("accepts the boundaries",
		func(limit int) {
			taskDAO.EXPECT().List(gomock.Any(), dao.ListOption{Limit: limit + 1}).Return([]model.Task{}, nil)
			Expect(doRequest(http.MethodGet, fmt.Sprintf("/tasks?limit=%d", limit), "").Code).To(Equal(http.StatusOK))
		},
		Entry("minimum", 1),
		Entry("maximum", MaxPageSize),
	)
})

var _ = Describe("ListTasks validation", func() {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	limitErr := errorBody{Code: CodeValidationFailed, Message: "limit: must be an integer between 1 and 100", Field: "limit"}
	nextKeyErr := errorBody{Code: CodeInvalidNextKey, Message: "invalid nextKey"}

	DescribeTable("rejects bad query parameters without touching the DAO",
		func(query string, status int, want errorBody) {
			w := doRequest(http.MethodGet, "/tasks?"+query, "")
			expectAPIError(w, status, want.Code)
			Expect(decode[errorResponse](w).Error).To(Equal(want))
		},
		Entry("non-integer limit", "limit=abc", http.StatusUnprocessableEntity, limitErr),
		Entry("zero limit", "limit=0", http.StatusUnprocessableEntity, limitErr),
		Entry("negative limit", "limit=-1", http.StatusUnprocessableEntity, limitErr),
		Entry("limit above max", "limit=101", http.StatusUnprocessableEntity, limitErr),
		Entry("next key not base64", "nextKey="+url.QueryEscape("not base64!"), http.StatusBadRequest, nextKeyErr),
		Entry("next key not json", "nextKey="+encode("garbage"), http.StatusBadRequest, nextKeyErr),
		Entry("next key bad cursor", "nextKey="+encode(`{"cursor":"not-a-uuid"}`), http.StatusBadRequest, nextKeyErr),
		Entry("next key without cursor", "nextKey="+encode(`{}`), http.StatusBadRequest, nextKeyErr),
		Entry("next key with null cursor", "nextKey="+encode(`{"cursor":null}`), http.StatusBadRequest, nextKeyErr),
		Entry("next key that is null", "nextKey="+encode(`null`), http.StatusBadRequest, nextKeyErr),
	)
})
