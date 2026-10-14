package zeroclaw

// The shared directory (<install>/shared/) and image uploads.
//
// Gateway handlers: crates/zeroclaw-gateway/src/api_browse.rs and
// api_upload.rs. Per-agent workspaces are in [AgentAPI].

import (
	"bytes"
	"context"
	"net/http"
)

// FilesAPI holds the shared-directory and upload endpoints, from
// [Client.Files].
type FilesAPI struct{ c *Client }

// Files returns the shared directory and uploads.
func (c *Client) Files() FilesAPI { return FilesAPI{c} }

// List returns a directory of the shared area, its root when path is empty
// (GET /api/browse).
func (a FilesAPI) List(ctx context.Context, path string) (*FileListing, error) {
	var out FileListing
	return &out, a.c.get(ctx, []string{"api", "browse"}, params("path", path), &out)
}

// MakeDir creates a directory (POST /api/browse/mkdir).
func (a FilesAPI) MakeDir(ctx context.Context, path string) error {
	return a.c.call(ctx, http.MethodPost, []string{"api", "browse", "mkdir"}, nil, map[string]string{"path": path}, nil)
}

// RemoveDir removes a directory and its contents (DELETE /api/browse/rmdir).
func (a FilesAPI) RemoveDir(ctx context.Context, path string) error {
	return a.c.call(ctx, http.MethodDelete, []string{"api", "browse", "rmdir"}, nil, map[string]string{"path": path}, nil)
}

// UploadImage stores an image (PNG, JPEG, GIF or WebP, told by its bytes) in
// agent's workspace and returns the marker that attaches it to a chat
// message (POST /api/upload).
func (a FilesAPI) UploadImage(ctx context.Context, agent string, image []byte) (*Upload, error) {
	req, err := a.c.request(ctx, http.MethodPost, []string{"api", "upload"}, params("agent", agent),
		bytes.NewReader(image), "application/octet-stream")
	if err != nil {
		return nil, err
	}
	var out Upload
	return &out, a.c.do(req, &out)
}

// FileListing is a directory's entries.
type FileListing struct {
	// Path is the directory.
	Path string `json:"path"`
	// Entries are the directories first, then the files, alphabetically.
	Entries []FileEntry `json:"entries"`
}

// FileEntry is a directory entry.
type FileEntry struct {
	// Name is the entry's name.
	Name string `json:"name"`
	// Kind is dir or file.
	Kind string `json:"kind"`
	// Size is the size of a file.
	Size *uint64 `json:"size,omitempty"`
	// Protected says whether it may not be changed.
	Protected bool `json:"protected"`
}

// FileContent is a file's content.
type FileContent struct {
	// Path is the file.
	Path string `json:"path"`
	// Size is its size in bytes.
	Size uint64 `json:"size"`
	// IsText says whether it is text.
	IsText bool `json:"is_text"`
	// Content is the content, in Encoding.
	Content string `json:"content"`
	// Encoding is utf8 or base64.
	Encoding string `json:"encoding"`
}

// Upload is a stored upload.
type Upload struct {
	// Path is where it was stored.
	Path string `json:"path"`
	// Marker is [IMAGE:<path>], to put in a message.
	Marker string `json:"marker"`
}
