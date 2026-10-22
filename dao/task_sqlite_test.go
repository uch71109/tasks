package dao

import (
	"context"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/uch71109/tasks/internal/testutil"
	"github.com/uch71109/tasks/model"
)

func newTestTask() model.Task {
	now := time.Now().UTC()
	return model.Task{
		ID:        uuid.Must(uuid.NewV7()),
		Name:      gofakeit.Word(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// expectStored asserts that the DAO returns want for its ID.
func expectStored(dao *SQLiteTaskDAO, want model.Task) {
	GinkgoHelper()
	got, err := dao.Get(context.Background(), want.ID)
	Expect(err).NotTo(HaveOccurred())
	Expect(got.ID).To(Equal(want.ID))
	Expect(got.Name).To(Equal(want.Name))
	Expect(got.Status).To(Equal(want.Status))
	Expect(got.CreatedAt).To(Equal(want.CreatedAt))
	Expect(got.UpdatedAt).To(Equal(want.UpdatedAt))
	Expect(got.DeletedAt).To(BeNil())
}

var _ = Describe("SQLiteTaskDAO", func() {
	var (
		ctx  context.Context
		dao  *SQLiteTaskDAO
		task model.Task
	)

	BeforeEach(func() {
		ctx = context.Background()
		dao = NewSQLiteTaskDAO(testutil.NewSQLiteDB())
		task = newTestTask()
	})

	Describe("Create", func() {
		It("stores the task", func() {
			Expect(dao.Create(ctx, task)).To(Succeed())
			expectStored(dao, task)
		})

		It("rejects a duplicate ID", func() {
			Expect(dao.Create(ctx, task)).To(Succeed())
			Expect(dao.Create(ctx, task)).NotTo(Succeed())
		})
	})

	Describe("Get", func() {
		It("returns the task with the given information", func() {
			other := newTestTask()
			Expect(dao.Create(ctx, task)).To(Succeed())
			Expect(dao.Create(ctx, other)).To(Succeed())
			expectStored(dao, task)
			expectStored(dao, other)
		})

		It("returns ErrTaskNotFound for an unknown ID", func() {
			_, err := dao.Get(ctx, uuid.New())
			Expect(err).To(MatchError(ErrTaskNotFound))
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			// An old timestamp lets the specs tell whether updated_at changes.
			task.CreatedAt = task.CreatedAt.Add(-time.Hour)
			task.UpdatedAt = task.CreatedAt
			task.Status = model.StatusCompleted
			Expect(dao.Create(ctx, task)).To(Succeed())
		})

		It("changes the given fields and returns the updated task", func() {
			name, status := gofakeit.Word(), model.StatusIncomplete
			got, err := dao.Update(ctx, task.ID, UpdateOption{Name: &name, Status: &status})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.ID).To(Equal(task.ID))
			Expect(got.Name).To(Equal(name))
			Expect(got.Status).To(Equal(status))
			Expect(got.CreatedAt).To(Equal(task.CreatedAt))
			Expect(got.UpdatedAt).To(BeTemporally("~", time.Now(), time.Second))
			expectStored(dao, got)
		})

		It("keeps the fields that are not given", func() {
			got, err := dao.Update(ctx, task.ID, UpdateOption{})
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Name).To(Equal(task.Name))
			Expect(got.Status).To(Equal(task.Status))
			Expect(got.CreatedAt).To(Equal(task.CreatedAt))
			Expect(got.UpdatedAt).To(BeTemporally("~", time.Now(), time.Second))
			expectStored(dao, got)
		})

		It("returns ErrTaskNotFound for an unknown ID", func() {
			_, err := dao.Update(ctx, uuid.New(), UpdateOption{})
			Expect(err).To(MatchError(ErrTaskNotFound))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			Expect(dao.Create(ctx, task)).To(Succeed())
			Expect(dao.Delete(ctx, task.ID)).To(Succeed())
		})

		It("keeps the record with deleted_at and updated_at set to now", func() {
			var deletedAt, updatedAt time.Time
			row := dao.db.QueryRow(`SELECT deleted_at, updated_at FROM tasks WHERE id = ?`, task.ID)
			Expect(row.Scan(&deletedAt, &updatedAt)).To(Succeed())
			Expect(deletedAt).To(BeTemporally("~", time.Now(), time.Second))
			Expect(updatedAt).To(Equal(deletedAt))
		})

		It("makes the task invisible to every operation", func() {
			_, err := dao.Get(ctx, task.ID)
			Expect(err).To(MatchError(ErrTaskNotFound))
			_, err = dao.Update(ctx, task.ID, UpdateOption{})
			Expect(err).To(MatchError(ErrTaskNotFound))
			Expect(dao.Delete(ctx, task.ID)).To(MatchError(ErrTaskNotFound))
			Expect(dao.List(ctx, ListOption{Limit: 10})).To(BeEmpty())
		})
	})

	Describe("List", func() {
		var tasks []model.Task

		BeforeEach(func() {
			tasks = make([]model.Task, 5)
			for i := range tasks {
				tasks[i] = newTestTask()
			}
			// Insert out of order to make sure List still orders by ID.
			for _, i := range []int{3, 0, 4, 1, 2} {
				Expect(dao.Create(ctx, tasks[i])).To(Succeed())
			}
			Expect(dao.Delete(ctx, tasks[1].ID)).To(Succeed())
		})

		DescribeTable("returns non-deleted tasks ordered by ID",
			func(opt func() ListOption, want []int) {
				expected := make([]model.Task, 0, len(want))
				for _, i := range want {
					expected = append(expected, tasks[i])
				}
				Expect(dao.List(ctx, opt())).To(Equal(expected))
			},
			Entry("all", func() ListOption { return ListOption{Limit: 10} }, []int{0, 2, 3, 4}),
			Entry("limit skips deleted", func() ListOption { return ListOption{Limit: 2} }, []int{0, 2}),
			Entry("nil cursor starts from beginning", func() ListOption { return ListOption{Cursor: uuid.Nil, Limit: 1} }, []int{0}),
			Entry("cursor is first", func() ListOption { return ListOption{Cursor: tasks[0].ID, Limit: 2} }, []int{2, 3}),
			Entry("cursor is deleted", func() ListOption { return ListOption{Cursor: tasks[1].ID, Limit: 10} }, []int{2, 3, 4}),
			Entry("cursor is last", func() ListOption { return ListOption{Cursor: tasks[4].ID, Limit: 10} }, []int{}),
		)
	})

	When("the database fails", func() {
		BeforeEach(func() {
			Expect(dao.db.Close()).To(Succeed())
		})

		It("returns the error from every method", func() {
			Expect(dao.Create(ctx, task)).NotTo(Succeed())
			_, err := dao.Get(ctx, task.ID)
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError(ErrTaskNotFound))
			_, err = dao.Update(ctx, task.ID, UpdateOption{})
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError(ErrTaskNotFound))
			err = dao.Delete(ctx, task.ID)
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError(ErrTaskNotFound))
			_, err = dao.List(ctx, ListOption{Limit: 10})
			Expect(err).To(HaveOccurred())
		})
	})
})
