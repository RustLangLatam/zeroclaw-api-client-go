package zeroclaw

// Response vectors: answers captured from a real gateway
// (scripts/capture-vectors.py, anonymized) must decode into the SDK's
// types, and re-encode without losing a field.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodes decodes raw as T and checks that encoding it back keeps every
// field raw has (a missing struct field would drop it).
func decodes[T any](raw json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var v T
	if err := dec.Decode(&v); err != nil {
		return err
	}
	again, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var before, after any
	json.Unmarshal(raw, &before)
	json.Unmarshal(again, &after)
	return kept("", before, after)
}

// kept reports the first non-null value of before that after lacks.
func kept(at string, before, after any) error {
	switch b := before.(type) {
	case map[string]any:
		a, ok := after.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: object became %v", at, after)
		}
		for k, v := range b {
			if err := kept(at+"."+k, v, a[k]); err != nil {
				return err
			}
		}
	case []any:
		a, ok := after.([]any)
		if !ok || len(a) != len(b) {
			return fmt.Errorf("%s: array became %v", at, after)
		}
		for i := range b {
			if err := kept(fmt.Sprintf("%s[%d]", at, i), b[i], a[i]); err != nil {
				return err
			}
		}
	case nil:
	default:
		if fmt.Sprint(before) != fmt.Sprint(after) {
			return fmt.Errorf("%s: %v became %v", at, before, after)
		}
	}
	return nil
}

type check func(json.RawMessage) error

// vectorTable maps every vector to the envelope field the SDK unwraps (if
// any) and the type that field decodes into.
var vectorTable = map[string]struct {
	field string
	check check
}{
	"health":               {"", decodes[Health]},
	"runtime_health":       {"health", decodes[HealthSnapshot]},
	"status":               {"", decodes[Status]},
	"channels":             {"channels", decodes[[]Channel]},
	"config_list":          {"", decodes[ConfigList]},
	"config_drift":         {"drifted", decodes[[]DriftEntry]},
	"config_reload_status": {"pending_reload", decodes[bool]},
	"config_templates":     {"templates", decodes[[]TemplateEntry]},
	"config_map_keys":      {"keys", decodes[[]string]},
	"config_catalog":       {"model_providers", decodes[[]CatalogModelProvider]},
	"config_status":        {"", decodes[ConfigStatus]},
	"config_agent_options": {"", decodes[AgentOptions]},
	"config_sections":      {"sections", decodes[[]ConfigSection]},
	"cron_list":            {"jobs", decodes[[]CronJob]},
	"cron_settings":        {"", decodes[CronSettings]},
	"cost":                 {"cost", decodes[CostSummary]},
	"sessions":             {"sessions", decodes[[]Session]},
	"events_history":       {"events", decodes[[]Event]},
	"tools":                {"tools", decodes[[]ToolSpec]},
	"integrations":         {"integrations", decodes[[]Integration]},
	"integration_settings": {"settings", decodes[map[string]IntegrationSetting]},
	"skill_bundles":        {"bundles", decodes[[]SkillBundle]},
	"slash_option_kinds":   {"kinds", decodes[[]json.RawMessage]},
	"quickstart_state":     {"", decodes[json.RawMessage]},
	"tuis":                 {"tuis", decodes[[]Tui]},
	"devices":              {"devices", decodes[[]Device]},
	"personality_files":    {"", decodes[PersonalityFiles]},
	"agent_skills":         {"", decodes[AgentSkills]},
}

type vector struct {
	Route    string                     `json:"route"`
	Build    string                     `json:"build"`
	Response map[string]json.RawMessage `json:"response"`
	raw      json.RawMessage
}

func vectors(t *testing.T) map[string]vector {
	t.Helper()
	files, err := filepath.Glob("testdata/vectors/*.json")
	if err != nil || len(files) < 20 {
		t.Fatalf("only %d vectors found: %v", len(files), err)
	}
	out := map[string]vector{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var v vector
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		v.raw = b
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = v
	}
	return out
}

func TestEveryCapturedAnswerDecodes(t *testing.T) {
	for name, v := range vectors(t) {
		entry, ok := vectorTable[name]
		if !ok {
			t.Errorf("%s: no type for this vector; add it to the table", name)
			continue
		}
		raw, _ := json.Marshal(v.Response)
		if entry.field != "" {
			raw = v.Response[entry.field]
		}
		if err := entry.check(raw); err != nil {
			t.Errorf("%s (%s): %v", name, v.Route, err)
		}
	}
}

func TestAnonymizationLeavesTheWireFormatIntact(t *testing.T) {
	for name, v := range vectors(t) {
		// A placeholder glued into a timestamp or a UUID means the
		// anonymizer rewrote something that was not a phone number.
		for _, artifact := range []string{".+34", "-+34", ":+34", "T+34"} {
			if bytes.Contains(v.raw, []byte(artifact)) {
				t.Errorf("%s holds %q", name, artifact)
			}
		}
	}
}

func TestVectorsRecordTheirRouteAndBuild(t *testing.T) {
	for name, v := range vectors(t) {
		if !strings.HasPrefix(v.Route, "/") || v.Build == "" {
			t.Errorf("%s: route %q build %q", name, v.Route, v.Build)
		}
	}
}

func TestALostFieldIsCaught(t *testing.T) {
	if decodes[TemplateEntry](json.RawMessage(`{"path":"a","kind":"map","not_modeled":1}`)) == nil {
		t.Error("a field the type does not model went unnoticed")
	}
	if err := decodes[TemplateEntry](json.RawMessage(`{"path":"a","kind":"map","value_type":null}`)); err != nil {
		t.Error(err)
	}
}
