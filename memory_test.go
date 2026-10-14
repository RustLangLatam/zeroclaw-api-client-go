package zeroclaw

import (
	"context"
	"net/http"
	"testing"
)

func TestMemoryStoresRecallsAndForgets(t *testing.T) {
	g, client := gateway(t)
	g.on("POST", "/api/memory", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"key":"k","content":"c","category":null,"agent":"helper"}`)
		reply(`{"status":"ok"}`)(w, r)
	})
	g.on("GET", "/api/memory", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "query", "c")
		wantQuery(t, r, "agent", "helper")
		reply(`{"entries":[{
			"id": "1", "key": "k", "content": "c", "category": "core", "timestamp": "t",
			"score": 0.9, "kind": { "semantic": "fact" }, "pinned": false
		}]}`)(w, r)
	})
	g.on("DELETE", "/api/memory/a%2Fb", reply(`{"status":"ok","deleted":true}`))

	ctx := context.Background()
	memory := client.Memory()
	if err := memory.Store(ctx, "k", "c", "", "helper"); err != nil {
		t.Fatal(err)
	}
	found := must[[]MemoryEntry](t)(memory.Search(ctx, MemoryQuery{Query: "c", Agent: "helper"}))
	if len(found) != 1 || found[0].Score == nil || *found[0].Score != 0.9 {
		t.Errorf("found %+v", found)
	}
	if !must[bool](t)(memory.Forget(ctx, "a/b", "")) {
		t.Error("not deleted")
	}
}
