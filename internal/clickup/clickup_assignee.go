package clickup

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	database "performance-dashboard-backend/internal/database"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"performance-dashboard-backend/internal/database/constants"
)

const memberCacheTTL = 5 * time.Minute

var (
	memberCacheMu      sync.RWMutex
	memberCacheByEmail map[string]*collectionmodels.Member
	memberCacheAt      time.Time
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// getMemberCache returns an email -> member map, reloading it from the staff
// member collection when the cached copy is older than memberCacheTTL. On a
// load failure the previous copy is kept so a transient DB problem does not
// silently change assignee resolution.
func getMemberCache() map[string]*collectionmodels.Member {
	memberCacheMu.RLock()
	cached, cachedAt := memberCacheByEmail, memberCacheAt
	memberCacheMu.RUnlock()
	if cached != nil && time.Since(cachedAt) < memberCacheTTL {
		return cached
	}

	members, err := database.GetMembersByTeam(
		os.Getenv("MONGO_URI"),
		os.Getenv("MONGODB_NAME"),
		os.Getenv("MONGODB_COLLECTION_STAFF_MEMBER"),
		"",
	)
	if err != nil {
		log.Printf("clickup: failed to load members for assignee resolution: %v", err)
		return cached
	}

	fresh := make(map[string]*collectionmodels.Member, len(members))
	for _, m := range members {
		if m == nil || m.Email == "" {
			continue
		}
		fresh[normalizeEmail(m.Email)] = m
	}

	memberCacheMu.Lock()
	memberCacheByEmail = fresh
	memberCacheAt = time.Now()
	memberCacheMu.Unlock()
	return fresh
}

// fallbackAssigneeEmail is the legacy positional heuristic: the first assignee
// for Concept tasks, the second one for every other team when present.
func fallbackAssigneeEmail(assignees []ClickUpAssignee, team string) string {
	if len(assignees) == 0 {
		return ""
	}
	idx := 0
	if team != constants.Concept && len(assignees) > 1 {
		idx = 1
	}
	return assignees[idx].Email
}

// resolveAssigneeEmail picks the assignee that actually belongs to team by
// matching each email against the staff member collection, instead of relying
// on the order ClickUp happens to return assignees in. Falls back to the
// positional heuristic when nothing matches.
func resolveAssigneeEmail(assignees []ClickUpAssignee, team string) string {
	if len(assignees) == 0 {
		return ""
	}
	if len(assignees) == 1 {
		return assignees[0].Email
	}

	if cache := getMemberCache(); cache != nil {
		for _, a := range assignees {
			member, ok := cache[normalizeEmail(a.Email)]
			if ok && strings.EqualFold(strings.TrimSpace(member.Team), team) {
				return a.Email
			}
		}
	}

	emails := make([]string, 0, len(assignees))
	for _, a := range assignees {
		emails = append(emails, a.Email)
	}
	fallback := fallbackAssigneeEmail(assignees, team)
	log.Printf("clickup: no assignee in [%s] matches team %s, falling back to %s",
		strings.Join(emails, ", "), team, fallback)
	return fallback
}
