package zeroclaw

// Configuration: per-property reads and writes, atomic JSON Patch batches,
// map aliases (agents, providers, channels…), drift, catalogs and the
// quickstart sections.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api_config.rs and
// api_sections.rs. Property paths are dotted
// (providers.models.anthropic.default.api_key). Reads return values as the
// gateway's display strings; writes take typed JSON, which the gateway
// coerces to the field's kind. Secrets are never returned: their reads say
// only whether they are populated.

import (
	"context"
	"encoding/json"
	"net/http"
)

// overrideDrift is the header that lets a [ConfigAPI.Patch] overwrite paths
// changed on disk since the gateway loaded them.
const overrideDriftHeader = "X-ZeroClaw-Override-Drift"

// ConfigAPI holds the configuration endpoints, from [Client.Config].
type ConfigAPI struct{ c *Client }

// Config returns the configuration endpoints (/api/config/...).
func (c *Client) Config() ConfigAPI { return ConfigAPI{c} }

// Snapshot returns the whole configuration with secrets masked as
// "***MASKED***" (GET /api/config). A compatibility snapshot; prefer
// [ConfigAPI.List] and [ConfigAPI.Get], which carry field metadata.
func (a ConfigAPI) Snapshot(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "config"}, nil, &out)
}

// Schema returns the JSON Schema of the whole configuration, static per
// gateway build (OPTIONS /api/config). Public.
func (a ConfigAPI) Schema(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodOptions, []string{"api", "config"}, nil, nil, &out)
}

// PropSchema returns the schema plus the field's metadata under
// x-zeroclaw-prop (OPTIONS /api/config/prop). Public.
func (a ConfigAPI) PropSchema(ctx context.Context, path string) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.call(ctx, http.MethodOptions, []string{"api", "config", "prop"}, params("path", path), nil, &out)
}

// Get reads one field (GET /api/config/prop).
func (a ConfigAPI) Get(ctx context.Context, path string) (*PropValue, error) {
	var out PropValue
	return &out, a.c.get(ctx, []string{"api", "config", "prop"}, params("path", path), &out)
}

// Set writes one field (PUT /api/config/prop). value is encoded as JSON
// (nil resets it); a non-empty comment is written above the key in the
// TOML. A map alias implied by the path is created.
func (a ConfigAPI) Set(ctx context.Context, path string, value any, comment string) (*PropValue, error) {
	body := map[string]any{"path": path, "value": value}
	if comment != "" {
		body["comment"] = comment
	}
	var out PropValue
	return &out, a.c.call(ctx, http.MethodPut, []string{"api", "config", "prop"}, nil, body, &out)
}

// Reset resets one field to its default (DELETE /api/config/prop).
func (a ConfigAPI) Reset(ctx context.Context, path string) (*PropValue, error) {
	var out PropValue
	return &out, a.c.call(ctx, http.MethodDelete, []string{"api", "config", "prop"}, params("path", path), nil, &out)
}

// Patch applies ops atomically, all or nothing (PATCH /api/config). With
// overrideDrift, paths edited on disk since the gateway loaded them are
// overwritten instead of refused with 409.
func (a ConfigAPI) Patch(ctx context.Context, ops []PatchOp, overrideDrift bool) (*PatchResponse, error) {
	req, err := a.c.jsonRequest(ctx, http.MethodPatch, []string{"api", "config"}, nil, ops)
	if err != nil {
		return nil, err
	}
	if overrideDrift {
		req.Header.Set(overrideDriftHeader, "true")
	}
	var out PatchResponse
	return &out, a.c.do(req, &out)
}

// List returns every field reachable under prefix (all fields when empty),
// with its kind, category and current value (GET /api/config/list).
func (a ConfigAPI) List(ctx context.Context, prefix string) (*ConfigList, error) {
	var out ConfigList
	return &out, a.c.get(ctx, []string{"api", "config", "list"}, params("prefix", prefix), &out)
}

// Drift returns the fields whose value on disk differs from the gateway's
// (GET /api/config/drift).
func (a ConfigAPI) Drift(ctx context.Context) ([]DriftEntry, error) {
	return callField[[]DriftEntry](ctx, a.c, http.MethodGet, []string{"api", "config", "drift"}, nil, nil, "drifted")
}

// PendingReload reports whether saved changes wait for a daemon reload
// ([SystemAPI.Reload]): GET /api/config/reload-status.
func (a ConfigAPI) PendingReload(ctx context.Context) (bool, error) {
	return callField[bool](ctx, a.c, http.MethodGet, []string{"api", "config", "reload-status"}, nil, nil, "pending_reload")
}

// Init fills unset nested sections under section (everywhere when empty)
// with defaults and returns the sections created (POST /api/config/init).
func (a ConfigAPI) Init(ctx context.Context, section string) ([]string, error) {
	return callField[[]string](ctx, a.c, http.MethodPost, []string{"api", "config", "init"}, params("section", section), nil, "initialized")
}

// Migrate migrates config.toml to the current schema in place, keeping a
// backup (POST /api/config/migrate).
func (a ConfigAPI) Migrate(ctx context.Context) (*MigrateResponse, error) {
	var out MigrateResponse
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "config", "migrate"}, nil, nil, &out)
}

// Templates returns the map and list sections and the type of their
// entries (GET /api/config/templates).
func (a ConfigAPI) Templates(ctx context.Context) ([]TemplateEntry, error) {
	return callField[[]TemplateEntry](ctx, a.c, http.MethodGet, []string{"api", "config", "templates"}, nil, nil, "templates")
}

// MapKeys returns the aliases of a map section, e.g. "agents" or
// "channels.whatsapp" (GET /api/config/map-keys).
func (a ConfigAPI) MapKeys(ctx context.Context, path string) ([]string, error) {
	return callField[[]string](ctx, a.c, http.MethodGet, []string{"api", "config", "map-keys"}, params("path", path), nil, "keys")
}

// CreateMapKey creates alias key under map path with defaults; Created is
// false when it already existed (POST /api/config/map-key).
func (a ConfigAPI) CreateMapKey(ctx context.Context, path, key string) (*MapKeyResponse, error) {
	var out MapKeyResponse
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "config", "map-key"}, params("path", path, "key", key), nil, &out)
}

// DeleteMapKey removes alias key and the references to it (DELETE
// /api/config/map-key). Deleting an agent also archives its workspace and
// its memory, cron jobs and sessions; check [ConfigAPI.DeletePlan] first.
func (a ConfigAPI) DeleteMapKey(ctx context.Context, path, key string) (*MapKeyResponse, error) {
	var out MapKeyResponse
	return &out, a.c.call(ctx, http.MethodDelete, []string{"api", "config", "map-key"}, params("path", path, "key", key), nil, &out)
}

// DeletePlan says what deleting alias key would block on or rewrite,
// without changing anything (GET /api/config/delete-plan).
func (a ConfigAPI) DeletePlan(ctx context.Context, path, key string) (*DeletePlan, error) {
	var out DeletePlan
	return &out, a.c.get(ctx, []string{"api", "config", "delete-plan"}, params("path", path, "key", key), &out)
}

// RenameMapKey renames alias from to to, and every reference to it (POST
// /api/config/rename-map-key).
func (a ConfigAPI) RenameMapKey(ctx context.Context, path, from, to string) (*RenameResponse, error) {
	var out RenameResponse
	body := map[string]string{"path": path, "from": from, "to": to}
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "config", "rename-map-key"}, nil, body, &out)
}

// RefreshContextWindow asks the provider for the model's context window and
// stores it (POST
// /api/config/model-providers/{type}/{alias}/refresh-context-window).
func (a ConfigAPI) RefreshContextWindow(ctx context.Context, providerType, alias string) (uint64, error) {
	return callField[uint64](ctx, a.c, http.MethodPost,
		[]string{"api", "config", "model-providers", providerType, alias, "refresh-context-window"}, nil, nil, "context_window")
}

// AliasValues returns the aliases an alias-reference field may point to;
// providers and channels as type.alias (GET
// /api/config/resolve-alias-source).
func (a ConfigAPI) AliasValues(ctx context.Context, source AliasSource) ([]string, error) {
	return callField[[]string](ctx, a.c, http.MethodGet, []string{"api", "config", "resolve-alias-source"},
		params("source", string(source)), nil, "values")
}

// Catalog returns the model providers this build knows (GET
// /api/config/catalog).
func (a ConfigAPI) Catalog(ctx context.Context) ([]CatalogModelProvider, error) {
	return callField[[]CatalogModelProvider](ctx, a.c, http.MethodGet, []string{"api", "config", "catalog"}, nil, nil, "model_providers")
}

// CatalogModels returns the models of a provider; it may call the provider
// with the configured credentials (GET /api/config/catalog/models).
func (a ConfigAPI) CatalogModels(ctx context.Context, modelProvider string) (*CatalogModels, error) {
	var out CatalogModels
	return &out, a.c.get(ctx, []string{"api", "config", "catalog", "models"}, params("model_provider", modelProvider), &out)
}

// Status reports whether the setup is complete (GET /api/config/status).
func (a ConfigAPI) Status(ctx context.Context) (*ConfigStatus, error) {
	var out ConfigStatus
	return &out, a.c.get(ctx, []string{"api", "config", "status"}, nil, &out)
}

// AgentOptions returns the aliases an agent can reference (GET
// /api/config/agent-options).
func (a ConfigAPI) AgentOptions(ctx context.Context) (*AgentOptions, error) {
	var out AgentOptions
	return &out, a.c.get(ctx, []string{"api", "config", "agent-options"}, nil, &out)
}

// Sections returns the setup sections, in wizard order (GET
// /api/config/sections).
func (a ConfigAPI) Sections(ctx context.Context) ([]ConfigSection, error) {
	return callField[[]ConfigSection](ctx, a.c, http.MethodGet, []string{"api", "config", "sections"}, nil, nil, "sections")
}

// SectionItems returns the items a picker section offers, e.g. the provider
// families of providers.models (GET /api/config/sections/{section}).
func (a ConfigAPI) SectionItems(ctx context.Context, section string) (*SectionItems, error) {
	var out SectionItems
	return &out, a.c.get(ctx, []string{"api", "config", "sections", section}, nil, &out)
}

// SelectSectionItem picks an item, creating what it needs, and returns where
// its fields live (POST /api/config/sections/{section}/items/{key}). An
// empty alias lets the gateway use "default" where the section takes one.
func (a ConfigAPI) SelectSectionItem(ctx context.Context, section, key, alias string) (*SelectedItem, error) {
	body := map[string]any{"alias": nil}
	if alias != "" {
		body["alias"] = alias
	}
	var out SelectedItem
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "config", "sections", section, "items", key}, nil, body, &out)
}

// PropValue is a field as GET, PUT and DELETE /api/config/prop return it:
// a regular field carries Value, a secret only Populated.
type PropValue struct {
	// Path is the dotted path.
	Path string `json:"path"`
	// Value is the gateway's display value (null after a reset); absent
	// for secrets.
	Value json.RawMessage `json:"value,omitempty"`
	// Warnings lists validation problems of the configuration.
	Warnings []ValidationWarning `json:"warnings,omitempty"`
	// Populated says, for a secret, whether it has a value.
	Populated *bool `json:"populated,omitempty"`
}

// IsSecret reports whether the field is a secret (only Populated is known).
func (p *PropValue) IsSecret() bool { return p.Populated != nil }

// ValidationWarning is a validation problem the gateway reports without
// refusing the write.
type ValidationWarning struct {
	// Code is stable, e.g. pre_existing_validation_error.
	Code string `json:"code"`
	// Message explains it.
	Message string `json:"message"`
	// Path is the field concerned.
	Path string `json:"path"`
}

// PatchOp is one operation of a [ConfigAPI.Patch]: RFC 6902 add, replace,
// remove and test, plus ZeroClaw's comment.
type PatchOp struct {
	// Op is add, replace, remove, test or comment.
	Op string `json:"op"`
	// Path is dotted, or a JSON Pointer (/a/b).
	Path string `json:"path"`
	// Value is the value for add, replace and test, encoded as JSON.
	Value any `json:"value,omitempty"`
	// Comment is the TOML comment written above the key.
	Comment string `json:"comment,omitempty"`
}

// ReplaceOp sets path to value.
func ReplaceOp(path string, value any) PatchOp {
	return PatchOp{Op: "replace", Path: path, Value: value}
}

// AddOp sets path to value, creating the map alias it implies.
func AddOp(path string, value any) PatchOp { return PatchOp{Op: "add", Path: path, Value: value} }

// RemoveOp resets path to its default.
func RemoveOp(path string) PatchOp { return PatchOp{Op: "remove", Path: path} }

// TestOp fails the whole patch unless path currently displays as value.
func TestOp(path string, value any) PatchOp { return PatchOp{Op: "test", Path: path, Value: value} }

// CommentOp writes comment above the key of path.
func CommentOp(path, comment string) PatchOp {
	return PatchOp{Op: "comment", Path: path, Comment: comment}
}

// PatchResponse is the result of a [ConfigAPI.Patch].
type PatchResponse struct {
	// Saved is always true on success: the patch was saved.
	Saved bool `json:"saved"`
	// Results holds one result per operation, in order.
	Results []PatchOpResult `json:"results"`
	// Warnings lists validation problems outside the patched paths.
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// PatchOpResult is what one patch operation left.
type PatchOpResult struct {
	// Op is the operation.
	Op string `json:"op"`
	// Path is the dotted path.
	Path string `json:"path"`
	// Value is the new display value (absent for secrets and comments).
	Value json.RawMessage `json:"value,omitempty"`
	// Populated says, for secrets, whether it now has a value.
	Populated *bool `json:"populated,omitempty"`
	// Comment is, for comment, the comment written.
	Comment *string `json:"comment,omitempty"`
}

// ConfigList is the result of [ConfigAPI.List].
type ConfigList struct {
	// Entries are the fields.
	Entries []ConfigEntry `json:"entries"`
	// Drifted lists the fields whose value on disk differs from the
	// gateway's.
	Drifted []DriftEntry `json:"drifted,omitempty"`
}

// ConfigEntry is one configuration field.
type ConfigEntry struct {
	// Path is the dotted path.
	Path string `json:"path"`
	// Category groups fields.
	Category string `json:"category"`
	// Kind is string, bool, integer, float, enum, alias-ref, string-array,
	// object-array or object.
	Kind string `json:"kind"`
	// TypeHint is the Rust type, e.g. Option<String>.
	TypeHint string `json:"type_hint"`
	// Value is the display value; absent for secrets.
	Value json.RawMessage `json:"value,omitempty"`
	// Populated says whether it has a value.
	Populated bool `json:"populated"`
	// IsSecret says whether it is a secret.
	IsSecret bool `json:"is_secret"`
	// IsEnvOverridden says whether an environment variable overrides it.
	IsEnvOverridden bool `json:"is_env_overridden"`
	// EnumVariants are the allowed values of an enum.
	EnumVariants []string `json:"enum_variants,omitempty"`
	// Section is the setup section it belongs to.
	Section *string `json:"section,omitempty"`
	// Tab is the dashboard tab.
	Tab string `json:"tab"`
	// Multiline says whether it is edited as multi-line text.
	Multiline bool `json:"multiline"`
}

// DriftEntry is a field whose value on disk differs from the gateway's.
type DriftEntry struct {
	// Path is the dotted path.
	Path string `json:"path"`
	// Secret says whether it is a secret (no values shown).
	Secret bool `json:"secret"`
	// Drifted is always true.
	Drifted bool `json:"drifted"`
	// InMemoryValue is the gateway's value.
	InMemoryValue json.RawMessage `json:"in_memory_value,omitempty"`
	// OnDiskValue is the value in config.toml.
	OnDiskValue json.RawMessage `json:"on_disk_value,omitempty"`
}

// MigrateResponse is the result of [ConfigAPI.Migrate].
type MigrateResponse struct {
	// Migrated says whether the file changed.
	Migrated bool `json:"migrated"`
	// BackupPath is where the previous file was kept.
	BackupPath *string `json:"backup_path,omitempty"`
	// SchemaVersion is the schema version now on disk.
	SchemaVersion int `json:"schema_version"`
}

// TemplateEntry is a map or list section and the type of its entries.
type TemplateEntry struct {
	// Path is the dotted path.
	Path string `json:"path"`
	// Kind is map or list.
	Kind string `json:"kind"`
	// ValueType is the Rust type of an entry.
	ValueType string `json:"value_type"`
	// Description says what the section holds.
	Description string `json:"description"`
}

// MapKeyResponse is the result of creating or deleting a map alias.
type MapKeyResponse struct {
	// Path is the map section.
	Path string `json:"path"`
	// Key is the alias.
	Key string `json:"key"`
	// Created says whether a new alias was created.
	Created bool `json:"created"`
	// Warnings lists cleanup steps that failed after the change was saved.
	Warnings []string `json:"warnings,omitempty"`
}

// DeletePlan says what deleting an alias would do.
type DeletePlan struct {
	// Path is the map section.
	Path string `json:"path"`
	// Key is the alias.
	Key string `json:"key"`
	// Allowed says whether the delete would go through.
	Allowed bool `json:"allowed"`
	// Blockers are the references that block it.
	Blockers []ReferenceSite `json:"blockers"`
	// Scrubs are the references it would clear.
	Scrubs []ReferenceSite `json:"scrubs"`
	// LiveACPSessions counts, for agents, the live ACP sessions (they block
	// the delete).
	LiveACPSessions *int `json:"live_acp_sessions,omitempty"`
	// CascadesOwnedState says whether the agent's memory, cron jobs and
	// sessions go too.
	CascadesOwnedState bool `json:"cascades_owned_state"`
}

// ReferenceSite is a field that references an alias.
type ReferenceSite struct {
	// Path is the dotted path of the referencing field.
	Path string `json:"path"`
	// RawValue is its raw value.
	RawValue string `json:"raw_value"`
}

// RenameResponse is the result of [ConfigAPI.RenameMapKey].
type RenameResponse struct {
	// Path is the map section.
	Path string `json:"path"`
	// From is the old alias.
	From string `json:"from"`
	// To is the new alias.
	To string `json:"to"`
	// Renamed says whether it was renamed.
	Renamed bool `json:"renamed"`
	// Warnings lists follow-up steps that failed (re-sending the rename
	// finishes them).
	Warnings []string `json:"warnings,omitempty"`
}

// AliasSource is an alias kind an alias-reference field can point to.
type AliasSource string

// The alias kinds.
const (
	AliasModelProviders         AliasSource = "model_providers" // providers.models.*
	AliasTTSProviders           AliasSource = "tts_providers"
	AliasTranscriptionProviders AliasSource = "transcription_providers"
	AliasChannels               AliasSource = "channels" // channels.*
	AliasRiskProfiles           AliasSource = "risk_profiles"
	AliasRuntimeProfiles        AliasSource = "runtime_profiles"
	AliasAgents                 AliasSource = "agents" // agents.*
	AliasSkillBundles           AliasSource = "skill_bundles"
	AliasKnowledgeBundles       AliasSource = "knowledge_bundles"
	AliasMCPBundles             AliasSource = "mcp_bundles"
)

// CatalogModelProvider is a model provider the build knows.
type CatalogModelProvider struct {
	// Name is the provider family name.
	Name string `json:"name"`
	// DisplayName is shown to people.
	DisplayName string `json:"display_name"`
	// Local says whether it runs locally.
	Local bool `json:"local"`
}

// CatalogModels lists the models of a provider.
type CatalogModels struct {
	// ModelProvider is the provider family name.
	ModelProvider string `json:"model_provider"`
	// Models are the model ids.
	Models []string `json:"models"`
	// Pricing holds the prices per model, when the provider publishes them.
	Pricing map[string]ModelPricing `json:"pricing,omitempty"`
	// Local says whether it runs locally.
	Local bool `json:"local"`
	// Live says whether the list came live from the provider.
	Live bool `json:"live"`
}

// ModelPricing is a model's prices as the provider states them.
type ModelPricing struct {
	// Prompt is the input price.
	Prompt *string `json:"prompt,omitempty"`
	// Completion is the output price.
	Completion *string `json:"completion,omitempty"`
	// InputCacheRead is the cached input read price.
	InputCacheRead *string `json:"input_cache_read,omitempty"`
	// InputCacheWrite is the cached input write price.
	InputCacheWrite *string `json:"input_cache_write,omitempty"`
}

// ConfigStatus says whether the setup is complete.
type ConfigStatus struct {
	// NeedsQuickstart says whether the quickstart still has to run.
	NeedsQuickstart bool `json:"needs_quickstart"`
	// Reason is has_dispatchable_agent, incomplete_agent or fresh_install.
	Reason string `json:"reason"`
	// HasPartialState says whether part of the setup is done.
	HasPartialState bool `json:"has_partial_state"`
	// Missing lists the steps left, in words.
	Missing []string `json:"missing"`
}

// AgentOptions lists the aliases an agent can reference.
type AgentOptions struct {
	// Channels, as type.alias.
	Channels []string `json:"channels"`
	// ChannelTypes are the channel types.
	ChannelTypes []string `json:"channel_types"`
	// ModelProviders, as type.alias.
	ModelProviders []string `json:"model_providers"`
	// RiskProfiles are the risk profiles.
	RiskProfiles []string `json:"risk_profiles"`
	// RuntimeProfiles are the runtime profiles.
	RuntimeProfiles []string `json:"runtime_profiles"`
	// SkillBundles are the skill bundles.
	SkillBundles []string `json:"skill_bundles"`
	// KnowledgeBundles are the knowledge bundles.
	KnowledgeBundles []string `json:"knowledge_bundles"`
	// MCPBundles are the MCP bundles.
	MCPBundles []string `json:"mcp_bundles"`
	// Agents are the agents.
	Agents []string `json:"agents"`
}

// ConfigSection is a setup section.
type ConfigSection struct {
	// Key is the section key, e.g. providers.models.
	Key string `json:"key"`
	// Label is shown to people.
	Label string `json:"label"`
	// Help is the help text.
	Help string `json:"help"`
	// HasPicker says whether it offers items to pick
	// ([ConfigAPI.SectionItems]).
	HasPicker bool `json:"has_picker"`
	// Completed says whether it is completed.
	Completed bool `json:"completed"`
	// Ready says whether it is ready to use.
	Ready bool `json:"ready"`
	// Group is the group it is shown under.
	Group string `json:"group"`
	// IsQuickstart says whether the quickstart includes it.
	IsQuickstart bool `json:"is_quickstart"`
	// Shape is direct_form, one_tier_alias_map, typed_family_map or
	// backend_picker.
	Shape *string `json:"shape,omitempty"`
	// CostCategory is the cost category.
	CostCategory string `json:"cost_category"`
}

// SectionItems lists the items of a picker section.
type SectionItems struct {
	// Section is the section key.
	Section string `json:"section"`
	// Items are the items.
	Items []SectionItem `json:"items"`
	// Help is the help text.
	Help string `json:"help"`
}

// SectionItem is an item of a picker section.
type SectionItem struct {
	// Key is the item key.
	Key string `json:"key"`
	// Label is shown to people.
	Label string `json:"label"`
	// Description describes it.
	Description *string `json:"description,omitempty"`
	// Badge is configured, needs setup, active or created.
	Badge *string `json:"badge,omitempty"`
}

// SelectedItem is the result of [ConfigAPI.SelectSectionItem].
type SelectedItem struct {
	// FieldsPrefix is the prefix of the fields to fill ([ConfigAPI.List]).
	FieldsPrefix string `json:"fields_prefix"`
	// Created says whether something was created.
	Created bool `json:"created"`
}
