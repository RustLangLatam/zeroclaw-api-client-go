package zeroclaw

// Scheduled jobs: shell commands or agent prompts on a cron schedule, with
// their results delivered to a channel.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api.rs (/api/cron*); types
// in crates/zeroclaw-runtime/src/cron/types.rs.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// cronLongRun is how long a manual run may take: the gateway allows 600 s.
const cronLongRun = 660 * time.Second

// CronAPI holds the cron endpoints, from [Client.Cron].
type CronAPI struct{ c *Client }

// Cron returns the scheduled jobs endpoints (/api/cron).
func (c *Client) Cron() CronAPI { return CronAPI{c} }

// List returns every job (GET /api/cron).
func (a CronAPI) List(ctx context.Context) ([]CronJob, error) {
	return callField[[]CronJob](ctx, a.c, http.MethodGet, []string{"api", "cron"}, nil, nil, "jobs")
}

// Create creates a job (POST /api/cron).
func (a CronAPI) Create(ctx context.Context, job *NewCronJob) (*CronJob, error) {
	return callField[*CronJob](ctx, a.c, http.MethodPost, []string{"api", "cron"}, nil, job, "job")
}

// Update changes a job; unset fields stay as they are (PATCH /api/cron/{id}).
func (a CronAPI) Update(ctx context.Context, id string, patch *CronJobPatch) (*CronJob, error) {
	return callField[*CronJob](ctx, a.c, http.MethodPatch, []string{"api", "cron", id}, nil, patch, "job")
}

// Delete deletes a job (DELETE /api/cron/{id}).
func (a CronAPI) Delete(ctx context.Context, id string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "cron", id}, nil, nil, nil)
}

// Run runs a job now and waits for it (POST /api/cron/{id}/run); the
// gateway allows up to its long-running timeout, 600 s by default. A failed
// job still returns no error, with Success false.
func (a CronAPI) Run(ctx context.Context, id string) (*CronRunResult, error) {
	req, err := a.c.request(withRequestTimeout(ctx, cronLongRun), http.MethodPost, []string{"api", "cron", id, "run"}, nil, nil, "")
	if err != nil {
		return nil, err
	}
	var out CronRunResult
	return &out, a.c.do(req, &out)
}

// Runs returns the job's latest runs (GET /api/cron/{id}/runs): limit 1 to
// 100, or 0 for the gateway's default, 20.
func (a CronAPI) Runs(ctx context.Context, id string, limit int) ([]CronRun, error) {
	var q []string
	if limit != 0 {
		q = []string{"limit", strconv.Itoa(limit)}
	}
	return callField[[]CronRun](ctx, a.c, http.MethodGet, []string{"api", "cron", id, "runs"}, params(q...), nil, "runs")
}

// Settings returns the scheduler settings (GET /api/cron/settings).
func (a CronAPI) Settings(ctx context.Context) (*CronSettings, error) {
	var out CronSettings
	return &out, a.c.get(ctx, []string{"api", "cron", "settings"}, nil, &out)
}

// UpdateSettings changes the scheduler settings that are set in settings;
// saved to the configuration (PATCH /api/cron/settings).
func (a CronAPI) UpdateSettings(ctx context.Context, settings *CronSettingsPatch) (*CronSettings, error) {
	var out CronSettings
	return &out, a.c.call(ctx, http.MethodPatch, []string{"api", "cron", "settings"}, nil, settings, &out)
}

// CronJob is a scheduled job.
type CronJob struct {
	// ID is the job id.
	ID string `json:"id"`
	// Expression is the schedule expression as written.
	Expression string `json:"expression"`
	// Schedule is the parsed schedule.
	Schedule Schedule `json:"schedule"`
	// Command is the shell command (shell jobs).
	Command string `json:"command"`
	// Prompt is the prompt (agent jobs).
	Prompt *string `json:"prompt,omitempty"`
	// Name is the name.
	Name *string `json:"name,omitempty"`
	// JobType is shell or agent.
	JobType string `json:"job_type"`
	// SessionTarget is isolated or main.
	SessionTarget string `json:"session_target"`
	// Model is the model override (agent jobs).
	Model *string `json:"model,omitempty"`
	// AgentAlias is the agent that owns it.
	AgentAlias string `json:"agent_alias"`
	// Enabled says whether it runs.
	Enabled bool `json:"enabled"`
	// Delivery is where its output goes.
	Delivery Delivery `json:"delivery"`
	// DeleteAfterRun says whether it is deleted after one run.
	DeleteAfterRun bool `json:"delete_after_run"`
	// AllowedTools lists the tools an agent job may use (nil: no
	// restriction).
	AllowedTools []string `json:"allowed_tools,omitempty"`
	// UsesMemory says whether an agent job uses memory.
	UsesMemory bool `json:"uses_memory"`
	// Source is imperative (API, CLI) or declarative (config file).
	Source string `json:"source"`
	// ShellOutputFormat is wrapped or raw (shell jobs).
	ShellOutputFormat *string `json:"shell_output_format,omitempty"`
	// CreatedAt is the creation time (RFC 3339).
	CreatedAt string `json:"created_at"`
	// NextRun is the next run (RFC 3339).
	NextRun string `json:"next_run"`
	// LastRun is the last run (RFC 3339).
	LastRun *string `json:"last_run,omitempty"`
	// LastStatus is the last status.
	LastStatus *string `json:"last_status,omitempty"`
	// LastOutput is the last output.
	LastOutput *string `json:"last_output,omitempty"`
}

// Schedule is when a job runs. Kind says which fields are set: cron (Expr,
// in Tz, an IANA zone, or UTC), at (At, RFC 3339) or every (EveryMs
// milliseconds). Build one with [CronSchedule], [AtSchedule] or
// [EverySchedule].
type Schedule struct {
	// Kind is cron, at or every.
	Kind string
	// Expr is the cron expression.
	Expr string
	// Tz is the time zone of a cron schedule (nil: UTC).
	Tz *string
	// At is the time of an at schedule (RFC 3339).
	At string
	// EveryMs is the interval of an every schedule.
	EveryMs uint64
}

// CronSchedule runs on the cron expression expr, in time zone tz (UTC when
// empty).
func CronSchedule(expr, tz string) Schedule {
	s := Schedule{Kind: "cron", Expr: expr}
	if tz != "" {
		s.Tz = &tz
	}
	return s
}

// AtSchedule runs once, at at (RFC 3339).
func AtSchedule(at string) Schedule { return Schedule{Kind: "at", At: at} }

// EverySchedule runs every ms milliseconds.
func EverySchedule(ms uint64) Schedule { return Schedule{Kind: "every", EveryMs: ms} }

// MarshalJSON encodes the schedule as the gateway does:
// {"kind":"cron","expr":…,"tz":…}, {"kind":"at","at":…} or
// {"kind":"every","every_ms":…}.
func (s Schedule) MarshalJSON() ([]byte, error) {
	switch s.Kind {
	case "cron":
		return json.Marshal(map[string]any{"kind": s.Kind, "expr": s.Expr, "tz": s.Tz})
	case "at":
		return json.Marshal(map[string]any{"kind": s.Kind, "at": s.At})
	case "every":
		return json.Marshal(map[string]any{"kind": s.Kind, "every_ms": s.EveryMs})
	}
	return nil, fmt.Errorf("unknown schedule kind %q", s.Kind)
}

// UnmarshalJSON decodes a schedule, refusing unknown kinds and missing
// fields.
func (s *Schedule) UnmarshalJSON(b []byte) error {
	var raw struct {
		Kind    string  `json:"kind"`
		Expr    *string `json:"expr"`
		Tz      *string `json:"tz"`
		At      *string `json:"at"`
		EveryMs *uint64 `json:"every_ms"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch {
	case raw.Kind == "cron" && raw.Expr != nil:
		*s = Schedule{Kind: raw.Kind, Expr: *raw.Expr, Tz: raw.Tz}
	case raw.Kind == "at" && raw.At != nil:
		*s = Schedule{Kind: raw.Kind, At: *raw.At}
	case raw.Kind == "every" && raw.EveryMs != nil:
		*s = Schedule{Kind: raw.Kind, EveryMs: *raw.EveryMs}
	default:
		return fmt.Errorf("invalid schedule %s", b)
	}
	return nil
}

// Delivery is where a job's output goes.
type Delivery struct {
	// Mode is none or announce (post it on a channel).
	Mode string `json:"mode"`
	// Channel is the channel, as type.alias.
	Channel *string `json:"channel,omitempty"`
	// To is the recipient on the channel.
	To *string `json:"to,omitempty"`
	// ThreadID is the thread to post in.
	ThreadID *string `json:"thread_id,omitempty"`
	// BestEffort says whether a delivery failure leaves the run successful
	// (true when absent from the JSON).
	BestEffort bool `json:"best_effort"`
}

// UnmarshalJSON decodes a delivery, BestEffort defaulting to true.
func (d *Delivery) UnmarshalJSON(b []byte) error {
	type plain Delivery
	out := plain{BestEffort: true}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*d = Delivery(out)
	return nil
}

// AnnounceDelivery posts the output to to on channel (type.alias).
func AnnounceDelivery(channel, to string) Delivery {
	return Delivery{Mode: "announce", Channel: &channel, To: &to, BestEffort: true}
}

// NewCronJob is a job to create. Shell jobs need Command; agent jobs Prompt
// (and JobType "agent", implied by a prompt).
type NewCronJob struct {
	// Agent is the owning agent (must be configured).
	Agent string `json:"agent"`
	// Schedule is the cron expression.
	Schedule string `json:"schedule"`
	// Name is the name.
	Name string `json:"name,omitempty"`
	// Tz is the IANA time zone of the schedule.
	Tz string `json:"tz,omitempty"`
	// Command is the shell command.
	Command string `json:"command,omitempty"`
	// JobType is shell or agent.
	JobType string `json:"job_type,omitempty"`
	// Prompt is the agent prompt.
	Prompt string `json:"prompt,omitempty"`
	// Delivery is where the output goes.
	Delivery *Delivery `json:"delivery,omitempty"`
	// SessionTarget is isolated (default) or main.
	SessionTarget string `json:"session_target,omitempty"`
	// Model is the model override.
	Model string `json:"model,omitempty"`
	// AllowedTools lists the tools an agent job may use (nil: no
	// restriction; a pointer to an empty list: none).
	AllowedTools *[]string `json:"allowed_tools,omitempty"`
	// DeleteAfterRun deletes the job after its first run.
	DeleteAfterRun *bool `json:"delete_after_run,omitempty"`
	// UsesMemory says whether an agent job uses memory (default true).
	UsesMemory *bool `json:"uses_memory,omitempty"`
	// ShellOutputFormat is wrapped (default) or raw: how a shell job's
	// output is announced.
	ShellOutputFormat string `json:"shell_output_format,omitempty"`
}

// CronJobPatch holds changes to a job; nil leaves a field as it is.
type CronJobPatch struct {
	// Agent is the owning agent; required when changing a shell job's
	// command.
	Agent *string `json:"agent,omitempty"`
	// Name is the name.
	Name *string `json:"name,omitempty"`
	// Schedule is the cron expression.
	Schedule *string `json:"schedule,omitempty"`
	// Tz is the IANA time zone.
	Tz *string `json:"tz,omitempty"`
	// ClearTz removes the time zone.
	ClearTz *bool `json:"clear_tz,omitempty"`
	// Command is the shell command (or an agent job's prompt).
	Command *string `json:"command,omitempty"`
	// Prompt is the agent prompt.
	Prompt *string `json:"prompt,omitempty"`
	// Enabled pauses (false) or resumes the job.
	Enabled *bool `json:"enabled,omitempty"`
	// UsesMemory says whether an agent job uses memory.
	UsesMemory *bool `json:"uses_memory,omitempty"`
	// ShellOutputFormat is wrapped or raw.
	ShellOutputFormat *string `json:"shell_output_format,omitempty"`
}

// CronRunResult is the result of [CronAPI.Run].
type CronRunResult struct {
	// Status is the run status.
	Status string `json:"status"`
	// JobID is the job id.
	JobID string `json:"job_id"`
	// Success says whether the job succeeded.
	Success bool `json:"success"`
	// Output is its output.
	Output string `json:"output"`
	// DurationMs is the duration.
	DurationMs int64 `json:"duration_ms"`
	// StartedAt is the start (RFC 3339).
	StartedAt string `json:"started_at"`
	// FinishedAt is the end (RFC 3339).
	FinishedAt string `json:"finished_at"`
}

// CronRun is a recorded run.
type CronRun struct {
	// ID is the run id.
	ID int64 `json:"id"`
	// JobID is the job id.
	JobID string `json:"job_id"`
	// StartedAt is the start (RFC 3339).
	StartedAt string `json:"started_at"`
	// FinishedAt is the end (RFC 3339).
	FinishedAt string `json:"finished_at"`
	// Status is the status.
	Status string `json:"status"`
	// Output is the output.
	Output *string `json:"output,omitempty"`
	// DurationMs is the duration.
	DurationMs *int64 `json:"duration_ms,omitempty"`
}

// CronSettings are the scheduler settings.
type CronSettings struct {
	// Enabled says whether the scheduler runs jobs.
	Enabled bool `json:"enabled"`
	// CatchUpOnStartup says whether missed runs run at startup.
	CatchUpOnStartup bool `json:"catch_up_on_startup"`
	// MaxRunHistory is the number of runs kept per job.
	MaxRunHistory uint32 `json:"max_run_history"`
}

// CronSettingsPatch holds changes to the scheduler settings; nil leaves a
// setting as it is.
type CronSettingsPatch struct {
	// Enabled says whether the scheduler runs jobs.
	Enabled *bool `json:"enabled,omitempty"`
	// CatchUpOnStartup says whether missed runs run at startup.
	CatchUpOnStartup *bool `json:"catch_up_on_startup,omitempty"`
	// MaxRunHistory is the number of runs kept per job.
	MaxRunHistory *uint32 `json:"max_run_history,omitempty"`
}
