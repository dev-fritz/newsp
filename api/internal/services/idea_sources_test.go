package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/dev-fritz/newsp/api/internal/repositories"
)

func TestWithCited(t *testing.T) {
	t.Parallel()
	c := func() repositories.Candidate { return repositories.Candidate{ItemID: uuid.New()} }
	a, b, x, y := c(), c(), c(), c()
	ids := func(list []repositories.Candidate) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(list))
		for _, v := range list {
			out = append(out, v.ItemID)
		}
		return out
	}

	assert.Equal(t, ids([]repositories.Candidate{x, y}), ids(withCited(nil, []repositories.Candidate{x, y}, 30)),
		"without a report the ranking is kept")
	assert.Equal(t, ids([]repositories.Candidate{a, b, x, y}), ids(withCited([]repositories.Candidate{a, b}, []repositories.Candidate{b, x, y}, 30)),
		"cited first, ranked items fill without repeats")
	assert.Equal(t, ids([]repositories.Candidate{a, b, x}), ids(withCited([]repositories.Candidate{a, b}, []repositories.Candidate{x, y}, 3)),
		"the limit holds")
}

func TestLeadingCited(t *testing.T) {
	t.Parallel()
	c := func() repositories.Candidate { return repositories.Candidate{ItemID: uuid.New()} }
	a, b, x, y := c(), c(), c(), c()

	assert.Equal(t, 0, leadingCited(nil, []repositories.Candidate{x, y}), "nothing cited")
	assert.Equal(t, 2, leadingCited([]repositories.Candidate{a, b}, withCited([]repositories.Candidate{a, b}, []repositories.Candidate{x, y}, 30)))
	assert.Equal(t, 1, leadingCited([]repositories.Candidate{a, b}, []repositories.Candidate{a, x, b}),
		"only the head counts")
}
