package service

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/uch71109/tasks/internal/daomock"
)

func TestService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Suite")
}

var (
	taskDAO *daomock.MockTaskDAO
	svc     *Service
	router  *gin.Engine
)

var _ = BeforeEach(func() {
	ctrl := gomock.NewController(GinkgoT())
	taskDAO = daomock.NewMockTaskDAO(ctrl)
	svc = &Service{TaskDAO: taskDAO}
	router = gin.New()
	svc.Register(router)
})

// doRequest sends a request through the routes of svc.
func doRequest(method, path, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, path, reader))
	return w
}

func decode[T any](w *httptest.ResponseRecorder) T {
	GinkgoHelper()
	var v T
	Expect(json.Unmarshal(w.Body.Bytes(), &v)).To(Succeed(), "body: %s", w.Body)
	return v
}

func expectAPIError(w *httptest.ResponseRecorder, status int, code ErrorCode) {
	GinkgoHelper()
	Expect(w.Code).To(Equal(status), "body: %s", w.Body)
	Expect(w.Header().Get("Content-Type")).To(HavePrefix("application/json"))
	Expect(decode[errorResponse](w).Error.Code).To(Equal(code))
}

// expectInternalError asserts a generic 500 that does not leak err's details.
func expectInternalError(w *httptest.ResponseRecorder, err error) {
	GinkgoHelper()
	expectAPIError(w, 500, CodeInternalError)
	Expect(w.Body.String()).NotTo(ContainSubstring(err.Error()))
}
