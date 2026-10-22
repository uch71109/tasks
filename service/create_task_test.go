package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/uch71109/tasks/model"
)

var _ = Describe("CreateTask", func() {
	var (
		name string
		body string
		w    *httptest.ResponseRecorder
	)

	JustBeforeEach(func() {
		w = doRequest(http.MethodPost, "/tasks", body)
	})

	BeforeEach(func() {
		name = gofakeit.Word()
		body = `{"name":"  ` + name + `  ","status":1}`
	})

	Describe("normal case", func() {
		var stored model.Task

		BeforeEach(func() {
			taskDAO.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task model.Task) error {
				stored = task
				return nil
			})
		})

		It("stores the task with a UUIDv7, a trimmed name and timestamps", func() {
			Expect(w.Code).To(Equal(http.StatusCreated))
			Expect(stored.ID.Version()).To(Equal(uuid.Version(7)))
			Expect(stored.Name).To(Equal(name))
			Expect(stored.Status).To(Equal(model.StatusCompleted))
			Expect(stored.CreatedAt).To(BeTemporally("~", time.Now(), time.Second))
			Expect(stored.UpdatedAt).To(Equal(stored.CreatedAt))
			Expect(stored.DeletedAt).To(BeNil())

			Expect(decode[model.Task](w)).To(Equal(stored))
			// deletedAt is omitted for live tasks.
			Expect(decode[map[string]any](w)).NotTo(HaveKey("deletedAt"))
		})
	})

	When("the status is 0", func() {
		BeforeEach(func() {
			body = `{"name":"a","status":0}`
			taskDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		})

		It("accepts it as incomplete rather than missing", func() {
			Expect(w.Code).To(Equal(http.StatusCreated))
			Expect(decode[model.Task](w).Status).To(Equal(model.StatusIncomplete))
		})
	})

	When("the name has 255 multi-byte characters", func() {
		BeforeEach(func() {
			body = `{"name":"` + strings.Repeat("任", 255) + `","status":0}`
			taskDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		})

		It("counts characters rather than bytes", func() {
			Expect(w.Code).To(Equal(http.StatusCreated))
		})
	})

	When("the DAO fails", func() {
		boom := errors.New("connection refused: secret-db:5432")

		BeforeEach(func() {
			taskDAO.EXPECT().Create(gomock.Any(), gomock.Any()).Return(boom)
		})

		It("returns a generic 500", func() {
			expectInternalError(w, boom)
		})
	})
})

var _ = Describe("CreateTask validation", func() {
	malformed := errorBody{Code: CodeInvalidJSON, Message: "malformed JSON"}
	wrongType := func(field string) errorBody {
		return errorBody{Code: CodeInvalidJSON, Message: field + ": has the wrong type", Field: field}
	}

	// A wrong type is reported by field only, without the decoder's Go type names.
	DescribeTable("rejects malformed bodies without touching the DAO",
		func(body string, want errorBody) {
			w := doRequest(http.MethodPost, "/tasks", body)
			expectAPIError(w, http.StatusBadRequest, CodeInvalidJSON)
			Expect(decode[errorResponse](w).Error).To(Equal(want))
		},
		Entry("empty body", ``, malformed),
		Entry("malformed json", `{"name":`, malformed),
		Entry("not an object", `[]`, malformed),
		Entry("wrong name type", `{"name":123,"status":0}`, wrongType("name")),
		Entry("wrong status type", `{"name":"a","status":"1"}`, wrongType("status")),
		Entry("fractional status", `{"name":"a","status":0.5}`, wrongType("status")),
	)

	It("rejects a body over the size limit with 413", func() {
		body := `{"name":"` + strings.Repeat("a", maxBodyBytes) + `","status":0}`
		expectAPIError(doRequest(http.MethodPost, "/tasks", body), http.StatusRequestEntityTooLarge, CodeRequestTooLarge)
	})

	invalid := func(field, message string) errorBody {
		return errorBody{Code: CodeValidationFailed, Message: field + ": " + message, Field: field}
	}
	nameTooLong := invalid("name", "must be at most 255 characters")
	badStatus := invalid("status", "must be 0 (incomplete) or 1 (completed)")

	DescribeTable("rejects invalid fields without touching the DAO",
		func(body string, want errorBody) {
			w := doRequest(http.MethodPost, "/tasks", body)
			expectAPIError(w, http.StatusUnprocessableEntity, CodeValidationFailed)
			Expect(decode[errorResponse](w).Error).To(Equal(want))
		},
		Entry("missing name", `{"status":0}`, invalid("name", "is required")),
		Entry("null name", `{"name":null,"status":0}`, invalid("name", "is required")),
		Entry("blank name", `{"name":" \t\n","status":0}`, invalid("name", "must not be empty")),
		Entry("name too long", `{"name":"`+strings.Repeat("a", 256)+`","status":0}`, nameTooLong),
		// The limit counts characters, not bytes.
		Entry("name too long in multi-byte characters", `{"name":"`+strings.Repeat("任", 256)+`","status":0}`, nameTooLong),
		Entry("missing status", `{"name":"a"}`, invalid("status", "is required")),
		Entry("null status", `{"name":"a","status":null}`, invalid("status", "is required")),
		Entry("negative status", `{"name":"a","status":-1}`, badStatus),
		Entry("status out of range", `{"name":"a","status":2}`, badStatus),
	)
})
