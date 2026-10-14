package zeroclaw

// Long-term memory: entries the agents store and recall.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api.rs (/api/memory*);
// entry type in crates/zeroclaw-api/src/memory_traits.rs.

import (
	"context"
	"encoding/json"
	"net/http"
)

// MemoryAPI holds the memory endpoints, from [Client.Memory].
type MemoryAPI struct{ c *Client }

// Memory returns the long-term memory endpoints (/api/memory).
func (c *Client) Memory() MemoryAPI { return MemoryAPI{c} }

// Search returns, with a text or time filter, the best matches (at most
// 50); otherwise every entry (of Category, when set) (GET /api/memory).
// Contents longer than 4096 characters come truncated.
func (a MemoryAPI) Search(ctx context.Context, filter MemoryQuery) ([]MemoryEntry, error) {
	q := params("query", filter.Query, "category", filter.Category, "since", filter.Since, "until", filter.Until, "agent", filter.Agent)
	return callField[[]MemoryEntry](ctx, a.c, http.MethodGet, []string{"api", "memory"}, q, nil, "entries")
}

// Store stores content under key (category core unless category is set),
// in agent's memory or, when agent is empty, the shared one
// (POST /api/memory).
func (a MemoryAPI) Store(ctx context.Context, key, content, category, agent string) error {
	body := map[string]any{"key": key, "content": content, "category": nil, "agent": nil}
	if category != "" {
		body["category"] = category
	}
	if agent != "" {
		body["agent"] = agent
	}
	return a.c.call(ctx, http.MethodPost, []string{"api", "memory"}, nil, body, nil)
}

// Forget forgets an entry of agent's memory (the shared one when agent is
// empty) and reports whether it existed (DELETE /api/memory/{key}).
func (a MemoryAPI) Forget(ctx context.Context, key, agent string) (bool, error) {
	return callField[bool](ctx, a.c, http.MethodDelete, []string{"api", "memory", key}, params("agent", agent), nil, "deleted")
}

// MemoryQuery holds the filters of [MemoryAPI.Search]; empty ones are not
// sent.
type MemoryQuery struct {
	// Query is the text to recall by.
	Query string
	// Category is an exact category (core, daily, conversation or custom).
	Category string
	// Since keeps entries at or after it (RFC 3339).
	Since string
	// Until keeps entries before it (RFC 3339).
	Until string
	// Agent searches an agent's memory instead of the shared one.
	Agent string
}

// MemoryEntry is a memory entry.
type MemoryEntry struct {
	// ID is the entry id.
	ID string `json:"id"`
	// Key is the key.
	Key string `json:"key"`
	// Content is the content.
	Content string `json:"content"`
	// Category is the category.
	Category string `json:"category"`
	// Timestamp is when it was stored.
	Timestamp string `json:"timestamp"`
	// SessionID is the session it came from.
	SessionID *string `json:"session_id,omitempty"`
	// Score is the relevance of a recall match.
	Score *float64 `json:"score,omitempty"`
	// Namespace is the namespace.
	Namespace *string `json:"namespace,omitempty"`
	// Importance is the importance.
	Importance *float64 `json:"importance,omitempty"`
	// SupersededBy is the entry that replaced it.
	SupersededBy *string `json:"superseded_by,omitempty"`
	// Kind is "episodic", "procedural" or {"semantic": "fact"|…}.
	Kind json.RawMessage `json:"kind,omitempty"`
	// Pinned says whether it is pinned.
	Pinned bool `json:"pinned"`
	// AgentAlias is the owning agent.
	AgentAlias *string `json:"agent_alias,omitempty"`
}
