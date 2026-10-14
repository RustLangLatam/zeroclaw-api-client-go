package zeroclaw

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

func TestFilesBrowsesSharedFilesAndUploadsImages(t *testing.T) {
	g, client := gateway(t)
	g.on("GET", "/api/browse", reply(`{"path":"","entries":[{"name":"x","kind":"dir"}]}`))
	g.on("POST", "/api/browse/mkdir", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"x"}`)
		reply(`{"created":"x"}`)(w, r)
	})
	g.on("DELETE", "/api/browse/rmdir", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"path":"x"}`)
		reply(`{"removed":"x"}`)(w, r)
	})
	g.on("POST", "/api/upload", func(w http.ResponseWriter, r *http.Request) {
		wantQuery(t, r, "agent", "helper")
		wantHeader(t, r, "Content-Type", "application/octet-stream")
		if got, _ := io.ReadAll(r.Body); !bytes.Equal(got, []byte("\x89PNG")) {
			t.Errorf("upload body = %q", got)
		}
		reply(`{"path":"/w/webui_1.png","marker":"[IMAGE:/w/webui_1.png]"}`)(w, r)
	})

	ctx, files := context.Background(), client.Files()
	if got := must[*FileListing](t)(files.List(ctx, "")); got.Entries[0].Kind != "dir" {
		t.Errorf("list: %+v", got)
	}
	if err := files.MakeDir(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if err := files.RemoveDir(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if got := must[*Upload](t)(files.UploadImage(ctx, "helper", []byte("\x89PNG"))); got.Marker != "[IMAGE:/w/webui_1.png]" {
		t.Errorf("upload: %+v", got)
	}
}
