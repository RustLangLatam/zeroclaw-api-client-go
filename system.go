package zeroclaw

// The gateway itself: health, status, metrics, diagnostics, tools,
// integrations, logs, version upgrades and daemon administration.
//
// Gateway handlers: crates/zeroclaw-gateway/src/lib.rs (/health, /metrics,
// /admin/*), api.rs, api_logs.rs, version.rs and api_sop_author.rs (tool
// parameter options).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// SystemAPI holds the gateway-wide endpoints, from [Client.System].
type SystemAPI struct{ c *Client }

// System returns the health, status, diagnostics, logs, versions and
// administration endpoints.
func (c *Client) System() SystemAPI { return SystemAPI{c} }

// Health returns liveness and component health (GET /health). Public.
func (a SystemAPI) Health(ctx context.Context) (*Health, error) {
	var out Health
	return &out, a.c.get(ctx, []string{"health"}, nil, &out)
}

// RuntimeHealth returns the runtime's component health (GET /api/health).
func (a SystemAPI) RuntimeHealth(ctx context.Context) (*HealthSnapshot, error) {
	out, err := callField[HealthSnapshot](ctx, a.c, http.MethodGet, []string{"api", "health"}, nil, nil, "health")
	return &out, err
}

// Metrics returns the Prometheus text exposition (GET /metrics). Public.
func (a SystemAPI) Metrics(ctx context.Context) (string, error) {
	req, err := a.c.request(ctx, http.MethodGet, []string{"metrics"}, nil, nil, "")
	if err != nil {
		return "", err
	}
	return a.c.text(req)
}

// Status returns the version, model, channels, nodes and process figures
// (GET /api/status); with a non-empty agent, that agent's model.
func (a SystemAPI) Status(ctx context.Context, agent string) (*Status, error) {
	var out Status
	return &out, a.c.get(ctx, []string{"api", "status"}, params("agent", agent), &out)
}

// Doctor runs the configuration diagnostics (POST /api/doctor).
func (a SystemAPI) Doctor(ctx context.Context) (*DoctorReport, error) {
	var out DoctorReport
	return &out, a.c.call(ctx, http.MethodPost, []string{"api", "doctor"}, nil, nil, &out)
}

// Tools returns the tools an agent (all of them when agent is empty) can
// call (GET /api/tools).
func (a SystemAPI) Tools(ctx context.Context, agent string) ([]ToolSpec, error) {
	return callField[[]ToolSpec](ctx, a.c, http.MethodGet, []string{"api", "tools"}, params("agent", agent), nil, "tools")
}

// ToolParamOptions returns the values a tool parameter of domain accepts
// (channel_refs, peer_targets, peer_groups, agent_aliases, tool_names,
// memory_categories): POST /api/tools/param-options. args holds the
// sibling arguments the options depend on (nil for none); an empty agent
// is sent as null.
func (a SystemAPI) ToolParamOptions(ctx context.Context, domain, agent string, args json.RawMessage) ([]ToolOption, error) {
	body := map[string]any{"domain": domain, "agent": nil, "args": args}
	if agent != "" {
		body["agent"] = agent
	}
	return callField[[]ToolOption](ctx, a.c, http.MethodPost, []string{"api", "tools", "param-options"}, nil, body, "options")
}

// Integrations returns the integrations this build offers (GET
// /api/integrations).
func (a SystemAPI) Integrations(ctx context.Context) ([]Integration, error) {
	return callField[[]Integration](ctx, a.c, http.MethodGet, []string{"api", "integrations"}, nil, nil, "integrations")
}

// IntegrationSettings returns each integration's state, by name (GET
// /api/integrations/settings).
func (a SystemAPI) IntegrationSettings(ctx context.Context) (map[string]IntegrationSetting, error) {
	return callField[map[string]IntegrationSetting](ctx, a.c, http.MethodGet, []string{"api", "integrations", "settings"}, nil, nil, "settings")
}

// CLITools returns the command-line tools found on the host's PATH (GET
// /api/cli-tools). Slow: the gateway runs each with --version.
func (a SystemAPI) CLITools(ctx context.Context) ([]CLITool, error) {
	return callField[[]CLITool](ctx, a.c, http.MethodGet, []string{"api", "cli-tools"}, nil, nil, "cli_tools")
}

// Tuis returns the terminal clients connected to the daemon (GET
// /api/tuis).
func (a SystemAPI) Tuis(ctx context.Context) ([]Tui, error) {
	return callField[[]Tui](ctx, a.c, http.MethodGet, []string{"api", "tuis"}, nil, nil, "tuis")
}

// Logs returns a page of structured log events, newest first (GET
// /api/logs). Continue with [LogQuery].UntilLineOffset set to the page's
// NextCursorLineOffset.
func (a SystemAPI) Logs(ctx context.Context, filter LogQuery) (*LogPage, error) {
	var out LogPage
	return &out, a.c.get(ctx, []string{"api", "logs"}, filter.values(), &out)
}

// VersionCheck reports whether a newer release exists (GET
// /api/version/check). force skips the gateway's one-hour cache; a
// non-empty version checks that one tag.
func (a SystemAPI) VersionCheck(ctx context.Context, force bool, version string) (*VersionCheck, error) {
	f := ""
	if force {
		f = "true"
	}
	var out VersionCheck
	return &out, a.c.get(ctx, []string{"api", "version", "check"}, params("force", f, "version", version), &out)
}

// Upgrade starts a self-upgrade (the latest release when version is empty)
// and returns the handoff id to follow with [SystemAPI.UpgradeStatus]
// (POST /api/version/upgrade). Needs gateway.allow_self_upgrade.
func (a SystemAPI) Upgrade(ctx context.Context, version string, autoRestart bool) (string, error) {
	body := map[string]any{"version": nil, "auto_restart": autoRestart}
	if version != "" {
		body["version"] = version
	}
	return callField[string](ctx, a.c, http.MethodPost, []string{"api", "version", "upgrade"}, nil, body, "handoff_id")
}

// UpgradeStatus returns the progress of an upgrade, the latest when
// handoffID is empty (GET /api/version/upgrade/status).
func (a SystemAPI) UpgradeStatus(ctx context.Context, handoffID string) (*UpgradeStatus, error) {
	var out UpgradeStatus
	return &out, a.c.get(ctx, []string{"api", "version", "upgrade", "status"}, params("handoff_id", handoffID), &out)
}

// Reload restarts the daemon's subsystems from the configuration on disk
// (POST /admin/reload). The connection drops shortly after; reconnect.
// Allowed from loopback, or remotely with gateway.allow_remote_admin.
func (a SystemAPI) Reload(ctx context.Context) (*AdminResponse, error) {
	var out AdminResponse
	return &out, a.c.call(ctx, http.MethodPost, []string{"admin", "reload"}, nil, nil, &out)
}

// Shutdown stops the gateway (POST /admin/shutdown). Loopback only.
func (a SystemAPI) Shutdown(ctx context.Context) (*AdminResponse, error) {
	var out AdminResponse
	return &out, a.c.call(ctx, http.MethodPost, []string{"admin", "shutdown"}, nil, nil, &out)
}

// OpenAPI returns the gateway's OpenAPI document, which covers the
// configuration, version, SOP and skill routes (GET /api/openapi.json).
// Public.
func (a SystemAPI) OpenAPI(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, []string{"api", "openapi.json"}, nil, &out)
}

// Health is the answer of GET /health.
type Health struct {
	// Status is always "ok" while the gateway answers.
	Status string `json:"status"`
	// Paired is whether at least one client is paired.
	Paired bool `json:"paired"`
	// RequirePairing is whether requests need a bearer token.
	RequirePairing bool `json:"require_pairing"`
	// Runtime is the component health (raw when the gateway could not
	// serialize it).
	Runtime json.RawMessage `json:"runtime"`
}

// HealthSnapshot is the runtime's component health.
type HealthSnapshot struct {
	// PID is the daemon process id.
	PID uint32 `json:"pid"`
	// UpdatedAt is when it was computed (RFC 3339).
	UpdatedAt string `json:"updated_at"`
	// UptimeSeconds is the time since the daemon started.
	UptimeSeconds uint64 `json:"uptime_seconds"`
	// Components is the health per component (gateway,
	// channel:whatsapp.x, …).
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth is one component's health.
type ComponentHealth struct {
	// Status is "ok", "error", …
	Status string `json:"status"`
	// UpdatedAt is the last update (RFC 3339).
	UpdatedAt string `json:"updated_at"`
	// LastOK is the last time it was healthy.
	LastOK *string `json:"last_ok,omitempty"`
	// LastError is the last error.
	LastError *string `json:"last_error,omitempty"`
	// RestartCount is how many times it was restarted.
	RestartCount uint64 `json:"restart_count"`
}

// Status is the answer of GET /api/status.
type Status struct {
	// Version is the ZeroClaw version.
	Version string `json:"version"`
	// ModelProvider is the model provider in use, as type.alias.
	ModelProvider *string `json:"model_provider,omitempty"`
	// Model is the model in use.
	Model string `json:"model"`
	// Temperature is the sampling temperature.
	Temperature *float64 `json:"temperature,omitempty"`
	// UptimeSeconds is the time since the daemon started.
	UptimeSeconds uint64 `json:"uptime_seconds"`
	// DaemonStartedAt is when the daemon started (RFC 3339).
	DaemonStartedAt string `json:"daemon_started_at"`
	// GatewayPort is the gateway port.
	GatewayPort uint16 `json:"gateway_port"`
	// Locale is the UI locale.
	Locale string `json:"locale"`
	// MemoryBackend is the memory backend.
	MemoryBackend string `json:"memory_backend"`
	// Paired is whether a client is paired.
	Paired bool `json:"paired"`
	// Channels are the configured channels, as type.alias.
	Channels map[string]bool `json:"channels"`
	// Nodes are the connected nodes and peers found on the network.
	Nodes json.RawMessage `json:"nodes,omitempty"`
	// Health is the component health.
	Health HealthSnapshot `json:"health"`
	// AgentAlias is the agent asked about, if any.
	AgentAlias *string `json:"agent_alias,omitempty"`
	// Process holds the process figures.
	Process *ProcessStats `json:"process,omitempty"`
	// CheckUpdates is whether the daemon checks for updates.
	CheckUpdates bool `json:"check_updates"`
	// AllowSelfUpgrade is whether self-upgrade is allowed.
	AllowSelfUpgrade bool `json:"allow_self_upgrade"`
	// RestartMode is "supervised", "self_respawn" or "manual".
	RestartMode *string `json:"restart_mode,omitempty"`
	// RestartHint says how to restart the daemon.
	RestartHint *string `json:"restart_hint,omitempty"`
	// LiveChannelConfig is the configuration a running channel picks up
	// without a restart.
	LiveChannelConfig []string `json:"live_channel_config,omitempty"`
}

// ProcessStats is the daemon's resource use.
type ProcessStats struct {
	// RSSBytes is the resident memory.
	RSSBytes uint64 `json:"rss_bytes"`
	// SystemRAMTotalBytes is the host memory.
	SystemRAMTotalBytes uint64 `json:"system_ram_total_bytes"`
	// CPUPercent is the CPU use.
	CPUPercent *float64 `json:"cpu_percent,omitempty"`
	// NumCPUs is the number of host CPUs.
	NumCPUs uint32 `json:"num_cpus"`
}

// DoctorReport is the result of [SystemAPI.Doctor].
type DoctorReport struct {
	// Results holds each check.
	Results []Diagnostic `json:"results"`
	// Summary counts the results by severity.
	Summary DoctorSummary `json:"summary"`
}

// Diagnostic is one diagnostic.
type Diagnostic struct {
	// Severity is "ok", "warn" or "error".
	Severity string `json:"severity"`
	// Category is the area checked.
	Category string `json:"category"`
	// Message is the finding.
	Message string `json:"message"`
}

// DoctorSummary holds the counts of a doctor run.
type DoctorSummary struct {
	// OK is the number of checks passed.
	OK int `json:"ok"`
	// Warnings is the number of warnings.
	Warnings int `json:"warnings"`
	// Errors is the number of errors.
	Errors int `json:"errors"`
}

// ToolSpec is a tool an agent can call.
type ToolSpec struct {
	// Name is the tool name.
	Name string `json:"name"`
	// Description says what it does.
	Description string `json:"description"`
	// Parameters is the JSON Schema of its arguments.
	Parameters json.RawMessage `json:"parameters"`
	// Output is the JSON Schema of its output, when declared.
	Output json.RawMessage `json:"output,omitempty"`
	// ParamDomains is the option domain of the parameters that take known
	// values.
	ParamDomains map[string]string `json:"param_domains,omitempty"`
}

// ToolOption is a value a tool parameter accepts.
type ToolOption struct {
	// Value is the value.
	Value string `json:"value"`
	// Label is the label to show.
	Label *string `json:"label,omitempty"`
	// Hint is an extra hint.
	Hint *string `json:"hint,omitempty"`
}

// Integration is an integration.
type Integration struct {
	// Name is the integration's name.
	Name string `json:"name"`
	// Description says what it does.
	Description string `json:"description"`
	// Category is "Chat", "AiModel", "ToolsAutomation" or "Platform".
	Category string `json:"category"`
	// CategoryLabel is the category in words.
	CategoryLabel string `json:"category_label"`
	// Status is "Available" or "Active".
	Status string `json:"status"`
}

// IntegrationSetting is an integration's state.
type IntegrationSetting struct {
	// Enabled is whether it is active.
	Enabled bool `json:"enabled"`
	// Category is its category.
	Category string `json:"category"`
	// Status is its status.
	Status string `json:"status"`
}

// CLITool is a command-line tool on the host.
type CLITool struct {
	// Name is the command name.
	Name string `json:"name"`
	// Path is the full path.
	Path string `json:"path"`
	// Version is the reported version.
	Version *string `json:"version,omitempty"`
	// Category is the kind of tool (VersionControl, Language, …).
	Category string `json:"category"`
}

// Tui is a terminal client connected to the daemon.
type Tui struct {
	// TuiID is the client id.
	TuiID string `json:"tui_id"`
	// ConnectedAt is when it connected (RFC 3339).
	ConnectedAt string `json:"connected_at"`
	// PeerLabel says who it is.
	PeerLabel string `json:"peer_label"`
	// Transport is "unix" or "wss".
	Transport string `json:"transport"`
}

// LogQuery holds the filters of [SystemAPI.Logs]. Empty strings, zero
// numbers and a nil UntilLineOffset are not sent.
type LogQuery struct {
	// SinceTS keeps events at or after (RFC 3339).
	SinceTS string
	// UntilTS keeps events before (RFC 3339).
	UntilTS string
	// UntilLineOffset is the paging cursor: the previous page's
	// NextCursorLineOffset.
	UntilLineOffset *uint64
	// Action is the event action.
	Action string
	// Category is the event category.
	Category string
	// Outcome is the event outcome.
	Outcome string
	// SeverityMin is the minimum severity number.
	SeverityMin int
	// TraceID is the trace id.
	TraceID string
	// Q is free text.
	Q string
	// HideInternal leaves out the daemon's internal events.
	HideInternal bool
	// Limit is the page size (gateway default 200).
	Limit int
	// Attributes are exact matches on attribution fields (agent_alias,
	// channel, session_key, tool, …).
	Attributes map[string]string
}

// values is the query string of the filters that are set.
func (q LogQuery) values() url.Values {
	v := params("since_ts", q.SinceTS, "until_ts", q.UntilTS, "action", q.Action,
		"category", q.Category, "outcome", q.Outcome, "trace_id", q.TraceID, "q", q.Q)
	if q.UntilLineOffset != nil {
		v.Set("until_line_offset", strconv.FormatUint(*q.UntilLineOffset, 10))
	}
	if q.SeverityMin != 0 {
		v.Set("severity_min", strconv.Itoa(q.SeverityMin))
	}
	if q.Limit != 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.HideInternal {
		v.Set("hide_internal", "true")
	}
	for k, a := range q.Attributes {
		v.Set(k, a)
	}
	return v
}

// LogPage is a page of [SystemAPI.Logs].
type LogPage struct {
	// Events are the log events (OpenTelemetry-like JSON, see ZeroClaw's
	// LogEvent).
	Events []json.RawMessage `json:"events"`
	// NextCursorLineOffset is the cursor for the next (older) page.
	NextCursorLineOffset *uint64 `json:"next_cursor_line_offset,omitempty"`
	// AtEnd is whether this is the oldest page.
	AtEnd bool `json:"at_end"`
	// DaemonStartedAt is when the daemon started.
	DaemonStartedAt string `json:"daemon_started_at"`
	// AttributionKeys are the attribution fields that can be filtered on.
	AttributionKeys []string `json:"attribution_keys,omitempty"`
}

// VersionCheck is the answer of [SystemAPI.VersionCheck].
type VersionCheck struct {
	// CurrentVersion is the running version.
	CurrentVersion string `json:"current_version"`
	// LatestVersion is the latest release (nil when the check failed).
	LatestVersion *string `json:"latest_version,omitempty"`
	// IsNewer is whether the latest is newer.
	IsNewer bool `json:"is_newer"`
	// ReleaseURL is the release page.
	ReleaseURL *string `json:"release_url,omitempty"`
	// ReleaseNotes are the release notes (Markdown).
	ReleaseNotes *string `json:"release_notes,omitempty"`
	// PublishedAt is the release date.
	PublishedAt *string `json:"published_at,omitempty"`
	// Error says why the check failed.
	Error *string `json:"error,omitempty"`
}

// UpgradeStatus is the progress of a self-upgrade.
type UpgradeStatus struct {
	// State is "idle", "running", "done", "restarting" or "failed".
	State string `json:"state"`
	// HandoffID is the upgrade id.
	HandoffID *string `json:"handoff_id,omitempty"`
	// Phase is the step, 0 to 6.
	Phase *uint8 `json:"phase,omitempty"`
	// LogTail holds the last log lines.
	LogTail []string `json:"log_tail,omitempty"`
	// PreviousVersion is the version before.
	PreviousVersion *string `json:"previous_version,omitempty"`
	// TargetVersion is the version being installed.
	TargetVersion *string `json:"target_version,omitempty"`
	// RestartMode is how the daemon restarts.
	RestartMode *string `json:"restart_mode,omitempty"`
	// RestartHint says how to restart it by hand.
	RestartHint *string `json:"restart_hint,omitempty"`
	// Error says why it failed.
	Error *string `json:"error,omitempty"`
}

// AdminResponse is the answer of the /admin actions.
type AdminResponse struct {
	// Success is whether it was accepted.
	Success bool `json:"success"`
	// Message says what happens next.
	Message string `json:"message"`
}
