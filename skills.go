package zeroclaw

// Skill bundles and the skills in them.
//
// Gateway handler: crates/zeroclaw-gateway/src/api_skills.rs. Bundles
// themselves are configuration (skill_bundles.<alias>); this file edits the
// skills inside them. A skill is a SKILL.md: front matter plus a Markdown
// body.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// SkillsAPI holds the skill endpoints, from [Client.Skills].
type SkillsAPI struct{ c *Client }

// Skills returns the skill bundles and skills (/api/skills).
func (c *Client) Skills() SkillsAPI { return SkillsAPI{c} }

// skillRoute is the route of skill name in bundle.
func skillRoute(bundle, name string) []string {
	return []string{"api", "skills", "bundles", bundle, "skills", name}
}

// Bundles returns the skill bundles (GET /api/skills/bundles).
func (a SkillsAPI) Bundles(ctx context.Context) ([]SkillBundle, error) {
	return callField[[]SkillBundle](ctx, a.c, http.MethodGet, []string{"api", "skills", "bundles"}, nil, nil, "bundles")
}

// List returns the skills of a bundle (GET
// /api/skills/bundles/{bundle}/skills).
func (a SkillsAPI) List(ctx context.Context, bundle string) ([]SkillSummary, error) {
	return callField[[]SkillSummary](ctx, a.c, http.MethodGet, []string{"api", "skills", "bundles", bundle, "skills"}, nil, nil, "skills")
}

// Create creates a skill, with scripts/, references/ and assets/ unless
// noScaffold (POST /api/skills/bundles/{bundle}/skills). frontmatter is
// encoded as JSON.
func (a SkillsAPI) Create(ctx context.Context, bundle, name string, frontmatter any, body string, noScaffold bool) error {
	in := map[string]any{"name": name, "frontmatter": frontmatter, "body": body, "no_scaffold": noScaffold}
	return a.c.call(ctx, http.MethodPost, []string{"api", "skills", "bundles", bundle, "skills"}, nil, in, nil)
}

// Read returns a skill (GET /api/skills/bundles/{bundle}/skills/{name}).
func (a SkillsAPI) Read(ctx context.Context, bundle, name string) (*Skill, error) {
	var out Skill
	return &out, a.c.get(ctx, skillRoute(bundle, name), nil, &out)
}

// Write replaces a skill's front matter (encoded as JSON) and body (PUT
// /api/skills/bundles/{bundle}/skills/{name}).
func (a SkillsAPI) Write(ctx context.Context, bundle, name string, frontmatter any, body string) error {
	in := map[string]any{"frontmatter": frontmatter, "body": body}
	return a.c.call(ctx, http.MethodPut, skillRoute(bundle, name), nil, in, nil)
}

// Delete archives a skill, or deletes it for good with purge (DELETE
// /api/skills/bundles/{bundle}/skills/{name}).
func (a SkillsAPI) Delete(ctx context.Context, bundle, name string, purge bool) error {
	return a.c.call(ctx, http.MethodDelete, skillRoute(bundle, name), params("purge", strconv.FormatBool(purge)), nil, nil)
}

// SlashOptionKinds returns the option types a skill's slash command may
// declare (GET /api/skills/slash-option-kinds).
func (a SkillsAPI) SlashOptionKinds(ctx context.Context) ([]json.RawMessage, error) {
	return callField[[]json.RawMessage](ctx, a.c, http.MethodGet, []string{"api", "skills", "slash-option-kinds"}, nil, nil, "kinds")
}

// SkillBundle is a skill bundle.
type SkillBundle struct {
	// Alias is the bundle's alias.
	Alias string `json:"alias"`
	// Directory is its directory.
	Directory string `json:"directory"`
	// Include lists the skills it includes.
	Include []string `json:"include,omitempty"`
	// Exclude lists the skills it leaves out.
	Exclude []string `json:"exclude,omitempty"`
}

// SkillSummary is a skill in a listing.
type SkillSummary struct {
	// Bundle is the bundle.
	Bundle string `json:"bundle"`
	// Name is the skill's name.
	Name string `json:"name"`
	// Directory is its directory.
	Directory string `json:"directory"`
	// Frontmatter is the front matter (name, description, tags,
	// slash_options, …).
	Frontmatter json.RawMessage `json:"frontmatter"`
}

// Skill is a skill.
type Skill struct {
	// Bundle is the bundle.
	Bundle string `json:"bundle"`
	// Name is the skill's name.
	Name string `json:"name"`
	// Frontmatter is the front matter.
	Frontmatter json.RawMessage `json:"frontmatter"`
	// Body is the Markdown body.
	Body string `json:"body"`
}
