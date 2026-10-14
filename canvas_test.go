package zeroclaw

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestCanvasPublishesFrames(t *testing.T) {
	frame := `{"frame_id":"f1","content_type":"markdown","content":"# Hi","timestamp":"t"}`
	g, client := gateway(t)
	g.on("GET", "/api/canvas", reply(`{"canvases":["board"]}`))
	g.on("POST", "/api/canvas/board", func(w http.ResponseWriter, r *http.Request) {
		wantBody(t, r, `{"content_type":"markdown","content":"# Hi"}`)
		replyStatus(201, `{"canvas_id":"board","frame":`+frame+`}`)(w, r)
	})
	g.on("GET", "/api/canvas/board", reply(`{"canvas_id":"board","frame":`+frame+`}`))
	g.on("GET", "/api/canvas/board/history", reply(`{"canvas_id":"board","frames":[`+frame+`]}`))
	g.on("DELETE", "/api/canvas/board", reply(`{"canvas_id":"board","status":"cleared"}`))

	ctx, canvas := context.Background(), client.Canvas()
	if got := must[[]string](t)(canvas.List(ctx)); !slices.Equal(got, []string{"board"}) {
		t.Errorf("list: %v", got)
	}
	if got := must[*CanvasFrame](t)(canvas.Publish(ctx, "board", "markdown", "# Hi")); got.FrameID != "f1" {
		t.Errorf("publish: %+v", got)
	}
	if got := must[*CanvasFrame](t)(canvas.Current(ctx, "board")); got.Content != "# Hi" {
		t.Errorf("current: %+v", got)
	}
	if got := must[[]CanvasFrame](t)(canvas.History(ctx, "board")); len(got) != 1 {
		t.Errorf("history: %+v", got)
	}
	if err := canvas.Clear(ctx, "board"); err != nil {
		t.Fatal(err)
	}
}
