package clickup

import (
	"testing"
	"time"

	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"performance-dashboard-backend/internal/database/constants"
)

// seedMemberCache installs a fake member cache so the tests never touch Mongo.
func seedMemberCache(t *testing.T, members ...*collectionmodels.Member) {
	t.Helper()
	cache := make(map[string]*collectionmodels.Member, len(members))
	for _, m := range members {
		cache[normalizeEmail(m.Email)] = m
	}
	memberCacheMu.Lock()
	memberCacheByEmail = cache
	memberCacheAt = time.Now()
	memberCacheMu.Unlock()
	t.Cleanup(func() {
		memberCacheMu.Lock()
		memberCacheByEmail = nil
		memberCacheAt = time.Time{}
		memberCacheMu.Unlock()
	})
}

func TestResolveAssigneeEmail(t *testing.T) {
	concept := &collectionmodels.Member{Email: "concept@ikameglobal.com", Team: constants.Concept}
	artist := &collectionmodels.Member{Email: "artist@ikameglobal.com", Team: constants.Art}

	tests := []struct {
		name      string
		assignees []ClickUpAssignee
		team      string
		want      string
	}{
		{
			name:      "concept member listed second",
			assignees: []ClickUpAssignee{{Email: artist.Email}, {Email: concept.Email}},
			team:      constants.Concept,
			want:      concept.Email,
		},
		{
			name:      "concept member listed first",
			assignees: []ClickUpAssignee{{Email: concept.Email}, {Email: artist.Email}},
			team:      constants.Concept,
			want:      concept.Email,
		},
		{
			name:      "art member listed first",
			assignees: []ClickUpAssignee{{Email: artist.Email}, {Email: concept.Email}},
			team:      constants.Art,
			want:      artist.Email,
		},
		{
			name:      "email case differs from database",
			assignees: []ClickUpAssignee{{Email: artist.Email}, {Email: "Concept@IkameGlobal.com"}},
			team:      constants.Concept,
			want:      "Concept@IkameGlobal.com",
		},
		{
			name:      "no match falls back to first for concept",
			assignees: []ClickUpAssignee{{Email: "ghost@ikameglobal.com"}, {Email: "other@ikameglobal.com"}},
			team:      constants.Concept,
			want:      "ghost@ikameglobal.com",
		},
		{
			name:      "no match falls back to second for other teams",
			assignees: []ClickUpAssignee{{Email: "ghost@ikameglobal.com"}, {Email: "other@ikameglobal.com"}},
			team:      constants.Video,
			want:      "other@ikameglobal.com",
		},
		{
			name:      "single assignee is used as is",
			assignees: []ClickUpAssignee{{Email: "solo@ikameglobal.com"}},
			team:      constants.Art,
			want:      "solo@ikameglobal.com",
		},
		{
			name:      "no assignees",
			assignees: nil,
			team:      constants.Concept,
			want:      "",
		},
	}

	seedMemberCache(t, concept, artist)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveAssigneeEmail(tc.assignees, tc.team); got != tc.want {
				t.Errorf("resolveAssigneeEmail() = %q, want %q", got, tc.want)
			}
		})
	}
}
