package uploads_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/scherepi/crossroads-backend/uploads"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func upload(t *testing.T, s *uploads.Store, field string, data []byte) (int, map[string]string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile(field, "shot.png")
	part.Write(data)
	form.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Upload(rec, req)
	out := map[string]string{}
	json.NewDecoder(rec.Body).Decode(&out)
	return rec.Code, out
}

func TestUploadStoresAndServesImage(t *testing.T) {
	s := &uploads.Store{Dir: t.TempDir(), BaseURL: "http://localhost:8080/uploads/"}
	data := pngBytes(t)

	code, out := upload(t, s, "file", data)
	if code != http.StatusCreated {
		t.Fatalf("status %d, %v", code, out)
	}
	if !strings.HasPrefix(out["url"], "http://localhost:8080/uploads/") || !strings.HasSuffix(out["url"], ".png") {
		t.Fatalf("url %q", out["url"])
	}

	// The same image again gets the same URL, and only one file exists.
	_, again := upload(t, s, "file", data)
	if again["url"] != out["url"] {
		t.Fatalf("re-upload gave %q, want %q", again["url"], out["url"])
	}
	entries, _ := os.ReadDir(s.Dir)
	if len(entries) != 1 {
		t.Fatalf("%d files stored, want 1", len(entries))
	}

	name := strings.TrimPrefix(out["url"], "http://localhost:8080/uploads/")
	rec := httptest.NewRecorder()
	s.Files().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+name, nil))
	got, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusOK || !bytes.Equal(got, data) {
		t.Fatalf("serving: status %d, %d bytes", rec.Code, len(got))
	}
	if rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestUploadRejectsNonImages(t *testing.T) {
	s := &uploads.Store{Dir: t.TempDir(), BaseURL: "http://x/uploads/"}
	for name, data := range map[string][]byte{
		"html": []byte("<!doctype html><script>alert(1)</script>"),
		"svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"text": []byte("just some text"),
	} {
		if code, _ := upload(t, s, "file", data); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", name, code)
		}
	}
	if entries, _ := os.ReadDir(s.Dir); len(entries) != 0 {
		t.Fatalf("%d files stored, want none", len(entries))
	}
}

func TestUploadRejectsTooBigAndMissingField(t *testing.T) {
	s := &uploads.Store{Dir: t.TempDir(), BaseURL: "http://x/uploads/"}
	big := append(pngBytes(t), make([]byte, uploads.MaxBytes)...)
	if code, _ := upload(t, s, "file", big); code != http.StatusRequestEntityTooLarge {
		t.Errorf("too big: status %d, want 413", code)
	}
	if code, _ := upload(t, s, "image", pngBytes(t)); code != http.StatusBadRequest {
		t.Errorf("wrong field: status %d, want 400", code)
	}
}

func TestFilesOnlyServesStoredNames(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/secret.txt", []byte("no"), 0o644)
	s := &uploads.Store{Dir: dir}
	for _, path := range []string{"/", "/secret.txt", "/../go.mod", "/" + strings.Repeat("a", 64) + ".html"} {
		rec := httptest.NewRecorder()
		s.Files().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}
