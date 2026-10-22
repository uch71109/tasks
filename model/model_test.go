package model

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestModel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model Suite")
}

var _ = Describe("Status", func() {
	DescribeTable("Valid",
		func(status Status, want bool) {
			Expect(status.Valid()).To(Equal(want))
		},
		Entry("incomplete", StatusIncomplete, true),
		Entry("completed", StatusCompleted, true),
		Entry("negative", Status(-1), false),
		Entry("out of range", Status(2), false),
	)
})
