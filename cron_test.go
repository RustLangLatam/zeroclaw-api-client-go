package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestScheduleVectors(t *testing.T) {
	vectors := []struct {
		json string
		want Schedule
	}{
		{`{"kind":"cron","expr":"0 9 * * *","tz":null}`, CronSchedule("0 9 * * *", "")},
		{`{"kind":"cron","expr":"*/5 * * * *","tz":"America/Bogota"}`, CronSchedule("*/5 * * * *", "America/Bogota")},
		{`{"kind":"at","at":"2026-10-14T08:00:00Z"}`, AtSchedule("2026-10-14T08:00:00Z")},
		{`{"kind":"every","every_ms":60000}`, EverySchedule(60_000)},
	}
	for _, v := range vectors {
		var got Schedule
		if err := json.Unmarshal([]byte(v.json), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, v.want) {
			t.Errorf("%s decodes as %+v, want %+v", v.json, got, v.want)
		}
		// Encoding gives back the gateway's JSON.
		if enc := must[[]byte](t)(json.Marshal(v.want)); !sameJSON(t, enc, []byte(v.json)) {
			t.Errorf("%+v encodes as %s, want %s", v.want, enc, v.json)
		}
	}
	var s Schedule
	if err := json.Unmarshal([]byte(`{"kind":"weekly"}`), &s); err == nil {
		t.Error("an unknown kind decodes")
	}
}

func TestDeliveryVectors(t *testing.T) {
	got := must[[]byte](t)(json.Marshal(AnnounceDelivery("whatsapp.main", "+34600000000")))
	if want := `{"mode":"announce","channel":"whatsapp.main","to":"+34600000000","best_effort":true}`; !sameJSON(t, got, []byte(want)) {
		t.Errorf("announce encodes as %s, want %s", got, want)
	}
	var lean Delivery
	if err := json.Unmarshal([]byte(`{"mode":"none"}`), &lean); err != nil {
		t.Fatal(err)
	}
	if !lean.BestEffort || lean.Channel != nil {
		t.Errorf("lean delivery: %+v", lean)
	}
}

func TestCronJobsDecodeFromTheGatewaysJSON(t *testing.T) {
	var job CronJob
	err := json.Unmarshal([]byte(`{
		"id": "j1", "expression": "0 9 * * 1-5",
		"schedule": { "kind": "cron", "expr": "0 9 * * 1-5", "tz": "Europe/Madrid" },
		"command": "", "prompt": "Daily summary", "name": "summary",
		"job_type": "agent", "session_target": "isolated", "model": null,
		"agent_alias": "helper", "enabled": true,
		"delivery": { "mode": "announce", "channel": "whatsapp.main", "to": "+34600000000", "best_effort": true },
		"delete_after_run": false, "uses_memory": true, "source": "imperative",
		"shell_output_format": "wrapped",
		"created_at": "2026-10-13T09:00:00Z", "next_run": "2026-10-14T07:00:00Z",
		"last_run": null, "last_status": null, "last_output": null
	}`), &job)
	if err != nil {
		t.Fatal(err)
	}
	if want := CronSchedule("0 9 * * 1-5", "Europe/Madrid"); !reflect.DeepEqual(job.Schedule, want) {
		t.Errorf("schedule = %+v, want %+v", job.Schedule, want)
	}
	if job.Delivery.Channel == nil || *job.Delivery.Channel != "whatsapp.main" {
		t.Errorf("delivery = %+v", job.Delivery)
	}
	must[[]byte](t)(json.Marshal(job))
}

const cronTestJob = `{
	"id": "j1", "expression": "0 9 * * *",
	"schedule": { "kind": "cron", "expr": "0 9 * * *", "tz": null },
	"command": "echo hi", "prompt": null, "name": null, "job_type": "shell",
	"session_target": "isolated", "model": null, "agent_alias": "helper", "enabled": true,
	"delivery": { "mode": "announce", "channel": "whatsapp.main", "to": "+34600000000", "best_effort": true },
	"delete_after_run": false, "uses_memory": true, "source": "imperative",
	"shell_output_format": "raw", "created_at": "t", "next_run": "t",
	"last_run": null, "last_status": null, "last_output": null
}`

func TestCronCreatesRunsAndRemovesAJob(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/cron", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{
			"agent": "helper", "schedule": "0 9 * * *", "command": "echo hi",
			"delivery": { "mode": "announce", "channel": "whatsapp.main", "to": "+34600000000", "best_effort": true },
			"shell_output_format": "raw"
		}`)
		reply(`{"status":"ok","job":`+cronTestJob+`}`)(w, r)
	})
	g.on("POST", "/api/cron/j1/run", reply(`{
		"status": "ok", "job_id": "j1", "success": true, "output": "hi",
		"duration_ms": 5, "started_at": "t", "finished_at": "t"
	}`))
	g.on("GET", "/api/cron/j1/runs", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "limit", "5")
		reply(`{"runs":[{
			"id": 1, "job_id": "j1", "started_at": "t", "finished_at": "t",
			"status": "ok", "output": "hi", "duration_ms": 5
		}]}`)(w, r)
	})
	g.on("PATCH", "/api/cron/j1", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"enabled":false}`)
		reply(`{"status":"ok","job":`+cronTestJob+`}`)(w, r)
	})
	g.on("DELETE", "/api/cron/j1", reply(`{"status":"ok"}`))
	g.on("GET", "/api/cron", reply(`{"jobs":[`+cronTestJob+`]}`))

	ctx := context.Background()
	cron := client.Cron()
	delivery := AnnounceDelivery("whatsapp.main", "+34600000000")
	created := must[*CronJob](t)(cron.Create(ctx, &NewCronJob{
		Agent:             "helper",
		Schedule:          "0 9 * * *",
		Command:           "echo hi",
		Delivery:          &delivery,
		ShellOutputFormat: "raw",
	}))
	if created.ID != "j1" {
		t.Errorf("created %q", created.ID)
	}
	if run := must[*CronRunResult](t)(cron.Run(ctx, "j1")); run.Output != "hi" {
		t.Errorf("run output %q", run.Output)
	}
	if runs := must[[]CronRun](t)(cron.Runs(ctx, "j1", 5)); len(runs) != 1 {
		t.Errorf("%d runs", len(runs))
	}
	disabled := false
	must[*CronJob](t)(cron.Update(ctx, "j1", &CronJobPatch{Enabled: &disabled}))
	if err := cron.Delete(ctx, "j1"); err != nil {
		t.Fatal(err)
	}
	if jobs := must[[]CronJob](t)(cron.List(ctx)); len(jobs) != 1 {
		t.Errorf("%d jobs", len(jobs))
	}
}

func TestCronSchedulerSettings(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/cron/settings", reply(`{"enabled":true,"catch_up_on_startup":false,"max_run_history":50}`))
	g.on("PATCH", "/api/cron/settings", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"max_run_history":10}`)
		reply(`{"status":"ok","enabled":true,"catch_up_on_startup":false,"max_run_history":10}`)(w, r)
	})

	ctx := context.Background()
	if s := must[*CronSettings](t)(client.Cron().Settings(ctx)); s.MaxRunHistory != 50 {
		t.Errorf("max_run_history %d", s.MaxRunHistory)
	}
	ten := uint32(10)
	if s := must[*CronSettings](t)(client.Cron().UpdateSettings(ctx, &CronSettingsPatch{MaxRunHistory: &ten})); s.MaxRunHistory != 10 {
		t.Errorf("max_run_history %d", s.MaxRunHistory)
	}
}
