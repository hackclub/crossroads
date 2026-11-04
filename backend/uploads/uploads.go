// Package uploads stores images people upload (project screenshots) on disk
// and serves them back.
//
// Files are named by the SHA-256 of their contents, so uploading the same
// image twice stores it once, and a file never changes once it exists. Only
// PNG, JPEG, GIF and WebP are accepted, judged by the bytes rather than the
// name or the browser's claimed type, so nothing that could run as a page
// (HTML, SVG) is ever served from here.
//
// This is a single-server setup: the files live in one directory. Running
// several instances, or a host with a throwaway disk, needs shared storage
// (an S3 bucket or the Hack Club CDN) instead. See planning/DEV_MODE.md.
package uploads

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/scherepi/crossroads-backend/respond"
)

// MaxBytes is the largest file accepted.
const MaxBytes = 5 << 20

var extensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type Store struct {
	Dir string // where files are written; created if missing
	// BaseURL is what file names are appended to in the URLs handed back,
	// like "https://crossroads.hackclub.com/uploads/". Project links have to
	// be absolute http(s) URLs, so this can't be a bare path.
	BaseURL string
}

// Upload takes a multipart form with one file in the "file" field and
// responds with {"url": "..."}. Mount it behind sign-in.
func (s *Store) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes+64<<10) // room for the form's own headers
	file, _, err := r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			respond.Error(w, http.StatusRequestEntityTooLarge, "That image is too big. The limit is 5 MB.")
			return
		}
		respond.Error(w, http.StatusBadRequest, `Send the image as a form field called "file".`)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "Couldn't read that image. Please try again.")
		return
	}
	if len(data) > MaxBytes {
		respond.Error(w, http.StatusRequestEntityTooLarge, "That image is too big. The limit is 5 MB.")
		return
	}
	ext, ok := extensions[http.DetectContentType(data)]
	if !ok {
		respond.Error(w, http.StatusUnprocessableEntity, "That isn't an image we can use. Upload a PNG, JPEG, GIF or WebP.")
		return
	}

	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + ext
	if err := s.save(name, data); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]string{"url": strings.TrimSuffix(s.BaseURL, "/") + "/" + name})
}

// save writes data to name unless it's already there, going through a
// temporary file so a half-written image is never served.
func (s *Store) save(name string, data []byte) error {
	path := filepath.Join(s.Dir, name)
	if _, err := os.Stat(path); err == nil {
		return nil // same contents, already stored
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Files serves stored images. Mount it at the path BaseURL points to, with
// that prefix stripped. There are no directory listings, and only names this
// package could have written are looked up.
func (s *Store) Files() http.Handler {
	files := http.FileServer(http.Dir(s.Dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !validName(name) {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'")
		h.Set("Cache-Control", "public, max-age=31536000, immutable") // names are content hashes
		files.ServeHTTP(w, r)
	})
}

func validName(name string) bool {
	hash, ext, ok := strings.Cut(name, ".")
	if !ok || len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		return false
	}
	for _, e := range extensions {
		if "."+ext == e {
			return true
		}
	}
	return false
}
