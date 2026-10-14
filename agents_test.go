package zeroclaw

import (
	"context"
	"net/http"
	"testing"
)

func TestAgentEditsItsWorkspace(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/agents/helper/workspace/list", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "docs")
		reply(`{"path":"docs","entries":[{"name":"a.pdf","kind":"file","size":10}]}`)(w, r)
	})
	g.on("GET", "/api/agents/helper/workspace/read", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "path", "docs/a.pdf")
		reply(`{"path":"docs/a.pdf","size":10,"is_text":false,"content":"JVBERg==","encoding":"base64"}`)(w, r)
	})
	g.on("POST", "/api/agents/helper/workspace/mkdir", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"docs"}`)
		reply(`{"created":"docs"}`)(w, r)
	})
	g.on("POST", "/api/agents/helper/workspace/move", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"from":"a","to":"b"}`)
		reply(`{"from":"a","to":"b"}`)(w, r)
	})
	g.on("DELETE", "/api/agents/helper/workspace/path", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"b"}`)
		reply(`{"removed":"b"}`)(w, r)
	})
	g.on("GET", "/api/agents/helper/skills", reply(`{"agent":"helper","skills":[{"name":"s","origin":"bundle"}]}`))

	ctx, agent := context.Background(), client.Agent("helper")
	if got := must[*FileListing](t)(agent.ListFiles(ctx, "docs")); got.Entries[0].Size == nil || *got.Entries[0].Size != 10 {
		t.Errorf("list: %+v", got)
	}
	if got := must[*FileContent](t)(agent.ReadFile(ctx, "docs/a.pdf")); got.Encoding != "base64" {
		t.Errorf("read: %+v", got)
	}
	for _, err := range []error{agent.MakeDir(ctx, "docs"), agent.MovePath(ctx, "a", "b"), agent.RemovePath(ctx, "b")} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := must[*AgentSkills](t)(agent.Skills(ctx)); len(got.Skills) != 1 {
		t.Errorf("skills: %+v", got)
	}
}

func TestAgentReadsAndWritesPersonalityFiles(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/personality", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		reply(`{"max_chars":20000,"files":[{"filename":"SOUL.md","exists":true,"size":3,"mtime_ms":1}]}`)(w, r)
	})
	g.on("GET", "/api/personality/SOUL.md", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		reply(`{"filename":"SOUL.md","content":"Hi","exists":true,"truncated":false,"mtime_ms":1}`)(w, r)
	})
	g.on("PUT", "/api/personality/SOUL.md", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		wantBody(t, r, `{"content":"New","expected_mtime_ms":1}`)
		reply(`{"bytes_written":3,"mtime_ms":2}`)(w, r)
	})
	g.on("PUT", "/api/personality/USER.md", replyStatus(409, `{"error":"personality_disk_drift","filename":"USER.md",
		"current_content":"x","current_mtime_ms":5}`))
	g.on("GET", "/api/personality/templates", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		wantQuery(t, r, "user_name", "Ana")
		reply(`{"preset":"default","files":[{"filename":"USER.md","content":"Ana"}]}`)(w, r)
	})

	ctx, agent := context.Background(), client.Agent("helper")
	if got := must[*PersonalityFiles](t)(agent.PersonalityFiles(ctx)); got.MaxChars != 20_000 {
		t.Errorf("files: %+v", got)
	}
	if got := must[*PersonalityFile](t)(agent.Personality(ctx, "SOUL.md")); got.Content == nil || *got.Content != "Hi" {
		t.Errorf("personality: %+v", got)
	}
	mtime := int64(1)
	if got := must[*PersonalityWritten](t)(agent.WritePersonality(ctx, "SOUL.md", "New", &mtime)); got.MtimeMs == nil || *got.MtimeMs != 2 {
		t.Errorf("write: %+v", got)
	}
	if _, err := agent.WritePersonality(ctx, "USER.md", "y", &mtime); StatusCode(err) != 409 {
		t.Errorf("drift: %v", err)
	}
	if got := must[*PersonalityTemplates](t)(agent.PersonalityTemplates(ctx, PersonalityContext{UserName: "Ana"})); got.Preset != "default" {
		t.Errorf("templates: %+v", got)
	}
}
