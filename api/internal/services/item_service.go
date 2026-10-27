package services

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/dev-fritz/newsp/api/internal/apperrors"
	"github.com/dev-fritz/newsp/api/internal/repositories"
)

// maxFeedQueryRunes bounds the full-text search input.
const maxFeedQueryRunes = 200

// Feed types come from the repository, which owns the SQL allowlist.
type (
	// ItemFilter narrows the item feed.
	ItemFilter = repositories.ItemFilter
	// ItemView is an item of the feed with its source and citation flag.
	ItemView = repositories.ItemView
	// ItemSort orders the feed.
	ItemSort = repositories.ItemSort
)

// Feed orderings.
const (
	ItemSortRecent     = repositories.ItemSortRecent
	ItemSortEngagement = repositories.ItemSortEngagement
)

// ItemService serves the item feed.
type ItemService struct {
	items ItemStore
}

// NewItemService builds an ItemService.
func NewItemService(items ItemStore) *ItemService {
	return &ItemService{items: items}
}

// List returns a page of the feed that any signed-in user can read.
func (s *ItemService) List(ctx context.Context, f ItemFilter, limit, offset int) ([]ItemView, int, error) {
	fields := map[string]string{}
	switch f.Sort {
	case "":
		f.Sort = ItemSortRecent
	case ItemSortRecent, ItemSortEngagement:
	default:
		fields["sort"] = "oneof"
	}
	if utf8.RuneCountInString(f.Query) > maxFeedQueryRunes {
		fields["q"] = "max"
	}
	if f.Since != nil && f.Until != nil && !f.Since.Before(*f.Until) {
		fields["since"] = "before_until"
	}
	if len(fields) > 0 {
		return nil, 0, apperrors.Validation("invalid query", fields)
	}
	items, total, err := s.items.List(ctx, f, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	return items, total, nil
}
