package zeroclaw

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSkillsManagesSkillsInBundles(t *testing.T) {
	g, client := gateway(t)
	const frontmatter = `{"name":"greet","description":"Greets"}`
	g.on("GET", "/api/skills/bundles", reply(`{"bundles":[{"alias":"team","directory":"/s/team","include":[],"exclude":[]}]}`))
	g.on("GET", "/api/skills/bundles/team/skills", reply(`{"skills":[
		{"bundle":"team","name":"greet","directory":"/s/team/greet","frontmatter":`+frontmatter+`}]}`))
	g.on("POST", "/api/skills/bundles/team/skills", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"name":"greet","frontmatter":`+frontmatter+`,"body":"","no_scaffold":true}`)
		replyStatus(201, `{"bundle":"team","name":"greet","directory":"/s/team/greet"}`)(w, r)
	})
	g.on("GET", "/api/skills/bundles/team/skills/greet",
		reply(`{"bundle":"team","name":"greet","frontmatter":`+frontmatter+`,"body":"# Greet"}`))
	g.on("PUT", "/api/skills/bundles/team/skills/greet", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"frontmatter":`+frontmatter+`,"body":"# Hi"}`)
		w.WriteHeader(http.StatusNoContent)
	})
	g.on("DELETE", "/api/skills/bundles/team/skills/greet", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "purge", "true")
		w.WriteHeader(http.StatusNoContent)
	})
	g.on("GET", "/api/skills/slash-option-kinds", reply(`{"kinds":[{"manifest_name":"string"}]}`))

	ctx, skills, fm := context.Background(), client.Skills(), json.RawMessage(frontmatter)
	if got := must[[]SkillBundle](t)(skills.Bundles(ctx)); got[0].Alias != "team" {
		t.Errorf("bundles: %+v", got)
	}
	if got := must[[]SkillSummary](t)(skills.List(ctx, "team")); got[0].Name != "greet" {
		t.Errorf("list: %+v", got)
	}
	if err := skills.Create(ctx, "team", "greet", fm, "", true); err != nil {
		t.Fatal(err)
	}
	if got := must[*Skill](t)(skills.Read(ctx, "team", "greet")); got.Body != "# Greet" {
		t.Errorf("read: %+v", got)
	}
	if err := skills.Write(ctx, "team", "greet", fm, "# Hi"); err != nil {
		t.Fatal(err)
	}
	if err := skills.Delete(ctx, "team", "greet", true); err != nil {
		t.Fatal(err)
	}
	if got := must[[]json.RawMessage](t)(skills.SlashOptionKinds(ctx)); len(got) != 1 {
		t.Errorf("kinds: %v", got)
	}
}
