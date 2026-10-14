package zeroclaw

// Per-agent resources: the agent's workspace files, its personality files
// and the skills it sees. Agents themselves are configuration entries
// (agents.<alias>), created and removed with [ConfigAPI].
//
// Gateway handlers: crates/zeroclaw-gateway/src/api_browse.rs,
// api_personality.rs and api_skills.rs. Workspace paths are relative to the
// agent's workspace; escaping it is refused, and the personality files and
// sessions at its root are protected.

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// AgentAPI holds one agent's endpoints, from [Client.Agent].
type AgentAPI struct {
	c     *Client
	alias string
}

// Agent returns the endpoints of agent alias.
func (c *Client) Agent(alias string) AgentAPI { return AgentAPI{c, alias} }

// workspace is the route of a workspace action.
func (a AgentAPI) workspace(action string) []string {
	return []string{"api", "agents", a.alias, "workspace", action}
}

// ListFiles returns a workspace directory, the root when path is empty (GET
// /api/agents/{alias}/workspace/list).
func (a AgentAPI) ListFiles(ctx context.Context, path string) (*FileListing, error) {
	var out FileListing
	return &out, a.c.get(ctx, a.workspace("list"), params("path", path), &out)
}

// ReadFile returns a workspace file, up to 4 MiB; binary files come
// base64-encoded (GET /api/agents/{alias}/workspace/read).
func (a AgentAPI) ReadFile(ctx context.Context, path string) (*FileContent, error) {
	var out FileContent
	return &out, a.c.get(ctx, a.workspace("read"), params("path", path), &out)
}

// MakeDir creates a directory (POST /api/agents/{alias}/workspace/mkdir).
func (a AgentAPI) MakeDir(ctx context.Context, path string) error {
	return a.c.call(ctx, http.MethodPost, a.workspace("mkdir"), nil, map[string]string{"path": path}, nil)
}

// MovePath moves or renames a file or directory (POST
// /api/agents/{alias}/workspace/move).
func (a AgentAPI) MovePath(ctx context.Context, from, to string) error {
	return a.c.call(ctx, http.MethodPost, a.workspace("move"), nil, map[string]string{"from": from, "to": to}, nil)
}

// RemovePath removes a file or directory (DELETE
// /api/agents/{alias}/workspace/path).
func (a AgentAPI) RemovePath(ctx context.Context, path string) error {
	return a.c.call(ctx, http.MethodDelete, a.workspace("path"), nil, map[string]string{"path": path}, nil)
}

// PersonalityFiles returns the agent's personality files and whether each
// exists (GET /api/personality).
func (a AgentAPI) PersonalityFiles(ctx context.Context) (*PersonalityFiles, error) {
	var out PersonalityFiles
	return &out, a.c.get(ctx, []string{"api", "personality"}, params("agent", a.alias), &out)
}

// Personality returns one personality file: SOUL.md, IDENTITY.md, USER.md,
// AGENTS.md, TOOLS.md, HEARTBEAT.md or MEMORY.md (GET
// /api/personality/{filename}).
func (a AgentAPI) Personality(ctx context.Context, filename string) (*PersonalityFile, error) {
	var out PersonalityFile
	return &out, a.c.get(ctx, []string{"api", "personality", filename}, params("agent", a.alias), &out)
}

// WritePersonality replaces a personality file, at most 20 000 characters
// (PUT /api/personality/{filename}). With a non-nil expectedMtimeMs, the
// write is refused (409) if the file changed since it was read.
func (a AgentAPI) WritePersonality(ctx context.Context, filename, content string, expectedMtimeMs *int64) (*PersonalityWritten, error) {
	body := map[string]any{"content": content, "expected_mtime_ms": expectedMtimeMs}
	var out PersonalityWritten
	return &out, a.c.call(ctx, http.MethodPut, []string{"api", "personality", filename}, params("agent", a.alias), body, &out)
}

// PersonalityContext fills the default personality files rendered by
// [AgentAPI.PersonalityTemplates]; empty fields are not sent.
type PersonalityContext struct {
	AgentName          string
	UserName           string
	Timezone           string
	CommunicationStyle string
	// IncludeMemory, when set, says whether to render MEMORY.md.
	IncludeMemory *bool
}

// PersonalityTemplates returns the default personality files rendered for
// this agent, without writing them (GET /api/personality/templates).
func (a AgentAPI) PersonalityTemplates(ctx context.Context, pc PersonalityContext) (*PersonalityTemplates, error) {
	var includeMemory string
	if pc.IncludeMemory != nil {
		includeMemory = strconv.FormatBool(*pc.IncludeMemory)
	}
	query := params("agent", a.alias, "agent_name", pc.AgentName, "user_name", pc.UserName,
		"timezone", pc.Timezone, "communication_style", pc.CommunicationStyle, "include_memory", includeMemory)
	var out PersonalityTemplates
	return &out, a.c.get(ctx, []string{"api", "personality", "templates"}, query, &out)
}

// Skills returns the skills the agent loads, where each comes from, and
// those dropped by the audit (GET /api/agents/{alias}/skills).
func (a AgentAPI) Skills(ctx context.Context) (*AgentSkills, error) {
	var out AgentSkills
	return &out, a.c.get(ctx, []string{"api", "agents", a.alias, "skills"}, nil, &out)
}

// PersonalityFiles lists the personality files of an agent.
type PersonalityFiles struct {
	// Files holds each editable file.
	Files []PersonalityFileEntry `json:"files"`
	// MaxChars is the maximum characters per file.
	MaxChars int `json:"max_chars"`
}

// PersonalityFileEntry is a personality file's presence.
type PersonalityFileEntry struct {
	// Filename is the file name.
	Filename string `json:"filename"`
	// Exists says whether it exists.
	Exists bool `json:"exists"`
	// Size is its size in bytes.
	Size uint64 `json:"size"`
	// MtimeMs is its modification time, ms since the epoch.
	MtimeMs *int64 `json:"mtime_ms,omitempty"`
}

// PersonalityFile is a personality file's content.
type PersonalityFile struct {
	// Filename is the file name.
	Filename string `json:"filename"`
	// Content is the content (absent when the file does not exist).
	Content *string `json:"content,omitempty"`
	// Exists says whether it exists.
	Exists bool `json:"exists"`
	// Truncated says whether the content was cut at the size limit.
	Truncated bool `json:"truncated"`
	// MtimeMs is its modification time, for [AgentAPI.WritePersonality].
	MtimeMs *int64 `json:"mtime_ms,omitempty"`
}

// PersonalityWritten is the result of [AgentAPI.WritePersonality].
type PersonalityWritten struct {
	// BytesWritten is the number of bytes written.
	BytesWritten uint64 `json:"bytes_written"`
	// MtimeMs is the new modification time.
	MtimeMs *int64 `json:"mtime_ms,omitempty"`
}

// PersonalityTemplates are the rendered default personality files.
type PersonalityTemplates struct {
	// Preset is the preset used.
	Preset string `json:"preset"`
	// Files holds each file: {filename, content}.
	Files []json.RawMessage `json:"files"`
}

// AgentSkills lists the skills an agent loads.
type AgentSkills struct {
	// Agent is the agent.
	Agent string `json:"agent"`
	// Skills are the loaded skills: name, description, origin (workspace,
	// open-skills, plugin, bundle), editable, …
	Skills []json.RawMessage `json:"skills"`
	// Dropped are the skills left out, with reason_kind and reason.
	Dropped []json.RawMessage `json:"dropped,omitempty"`
}
