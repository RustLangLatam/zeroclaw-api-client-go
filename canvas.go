package zeroclaw

// Canvases: HTML, SVG, Markdown or text frames an agent (or a client)
// publishes for display.
//
// Gateway handler: crates/zeroclaw-gateway/src/canvas.rs. Limits: 100
// canvases, 50 frames of history each, 256 KiB per frame (the gateway's
// 64 KiB request limit applies first).

import (
	"context"
	"net/http"
)

// CanvasAPI holds the canvas endpoints, from [Client.Canvas].
type CanvasAPI struct{ c *Client }

// Canvas returns the canvas endpoints (/api/canvas).
func (c *Client) Canvas() CanvasAPI { return CanvasAPI{c} }

// List returns the canvas ids (GET /api/canvas).
func (a CanvasAPI) List(ctx context.Context) ([]string, error) {
	return callField[[]string](ctx, a.c, http.MethodGet, []string{"api", "canvas"}, nil, nil, "canvases")
}

// Current returns the current frame (GET /api/canvas/{id}).
func (a CanvasAPI) Current(ctx context.Context, id string) (*CanvasFrame, error) {
	return callField[*CanvasFrame](ctx, a.c, http.MethodGet, []string{"api", "canvas", id}, nil, nil, "frame")
}

// Publish publishes a frame, creating the canvas (POST /api/canvas/{id}).
// contentType is html, svg, markdown or text.
func (a CanvasAPI) Publish(ctx context.Context, id, contentType, content string) (*CanvasFrame, error) {
	return callField[*CanvasFrame](ctx, a.c, http.MethodPost, []string{"api", "canvas", id}, nil,
		map[string]string{"content_type": contentType, "content": content}, "frame")
}

// History returns past frames, oldest first (GET /api/canvas/{id}/history).
func (a CanvasAPI) History(ctx context.Context, id string) ([]CanvasFrame, error) {
	return callField[[]CanvasFrame](ctx, a.c, http.MethodGet, []string{"api", "canvas", id, "history"}, nil, nil, "frames")
}

// Watch follows a canvas live (GET /ws/canvas/{id}). Frames are
// {"type": "connected" | "frame" | "lagged" | "error", …}; a frame carries
// the published frame.
func (a CanvasAPI) Watch(ctx context.Context, id string) (*Feed, error) {
	return a.c.feed(ctx, "ws", "canvas", id)
}

// Clear clears a canvas (DELETE /api/canvas/{id}).
func (a CanvasAPI) Clear(ctx context.Context, id string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "canvas", id}, nil, nil, nil)
}

// CanvasFrame is a canvas frame.
type CanvasFrame struct {
	// FrameID is the frame id.
	FrameID string `json:"frame_id"`
	// ContentType is html, svg, markdown or text.
	ContentType string `json:"content_type"`
	// Content is the content.
	Content string `json:"content"`
	// Timestamp is when it was published.
	Timestamp string `json:"timestamp"`
}
