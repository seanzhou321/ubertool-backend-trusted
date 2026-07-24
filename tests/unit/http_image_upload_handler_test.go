package unit

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	httpapi "ubertool-backend-trusted/internal/api/http"
	"ubertool-backend-trusted/internal/storage"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// newMockUploadRouterForTest wires the same routes cmd/server/main.go registers for local
// ("mock") storage mode — a second, plain net/http listener that never passes through the gRPC
// authInterceptor chain — backed by a temp directory instead of the real uploads/ tree.
func newMockUploadRouterForTest(t *testing.T) (router *mux.Router, uploadsDir string) {
	t.Helper()
	uploadsDir = t.TempDir()
	mockStorage, err := storage.NewMockStorageService("http://localhost:0", uploadsDir)
	require.NoError(t, err)

	router = mux.NewRouter()
	httpapi.RegisterMockStorageRoutes(router, mockStorage)
	return router, uploadsDir
}

// TestImageUploadHandler_UnauthenticatedWrite exposes SEC-HTTP-001 (sbr/rtm/009-security.rtm.md):
// HandleMockUpload is reachable on its own net/http listener, entirely outside the gRPC auth
// interceptor chain (cmd/server/main.go registers it on a separate http.ListenAndServe that
// never wraps authInterceptor). GeneratePresignedUploadURL mints a per-upload token
// (internal/storage/mock_storage.go:54) embedded in the URL path — implying an intended "only
// the holder of a freshly issued token may write" contract — but HandleMockUpload
// (internal/api/http/image_upload_handler.go:26-56) never reads that path segment at all, only
// the `key` query parameter, so a request bearing a token the server never issued succeeds
// identically to a legitimate upload.
func TestImageUploadHandler_UnauthenticatedWrite(t *testing.T) {
	router, uploadsDir := newMockUploadRouterForTest(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/upload/never-issued-token?key=tools/1/1/f.jpg", strings.NewReader("fake-image-bytes"))
	req.Header.Set("Content-Type", "image/jpeg")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusOK, rec.Code,
		"a request bearing a token the server never issued must be rejected — instead it was accepted (status %d) and the file was written, proving the upload token in the URL path is decorative", rec.Code)

	_, statErr := os.Stat(filepath.Join(uploadsDir, "images", "tools", "1", "1", "f.jpg"))
	require.Error(t, statErr, "no file should have been written for an unauthenticated/unauthorized upload")
}

// TestImageUploadHandler_UnauthenticatedRead is the read-side counterpart of
// TestImageUploadHandler_UnauthenticatedWrite (SEC-HTTP-001): HandleMockDownload
// (internal/api/http/image_upload_handler.go:59-98) also never validates the token path
// segment, so any stored image — including ones belonging to a tool the caller has no
// relationship to — can be fetched by anyone who can guess or enumerate its `key`.
func TestImageUploadHandler_UnauthenticatedRead(t *testing.T) {
	router, uploadsDir := newMockUploadRouterForTest(t)

	secretPath := filepath.Join(uploadsDir, "images", "tools", "42", "99")
	require.NoError(t, os.MkdirAll(secretPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(secretPath, "private.jpg"), []byte("someone else's photo"), 0644))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download/never-issued-token?key=tools/42/99/private.jpg", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusOK, rec.Code,
		"a download request bearing a token the server never issued must be rejected — instead it returned status %d with the file's bytes", rec.Code)
}

// TestImageUploadHandler_PathTraversal exposes SEC-HTTP-002: the `key` query parameter flows
// unsanitized into filepath.Join(imagesDir, key) (internal/storage/mock_storage.go:81,115), so a
// key containing ".." escapes the intended uploads/images directory entirely.
func TestImageUploadHandler_PathTraversal(t *testing.T) {
	router, uploadsDir := newMockUploadRouterForTest(t)

	// One ".." cancels the "images" path segment, landing the write in uploadsDir itself —
	// outside the "images" sandbox HandleMockUpload/MockStorageService.SaveFile are meant to be
	// confined to, while still staying inside this test's own t.TempDir() so nothing escapes
	// onto the real filesystem even in the vulnerable case.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/upload/some-token?key=../traversal-poc.txt", strings.NewReader("escaped the images sandbox"))
	req.Header.Set("Content-Type", "image/jpeg")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	escapedPath := filepath.Join(uploadsDir, "traversal-poc.txt") // one level above imagesDir = uploadsDir/images
	_, statErr := os.Stat(escapedPath)
	require.Error(t, statErr,
		"a key containing '..' must not be able to write outside the images directory — but the file landed at %s, one level above %s/images", escapedPath, uploadsDir)
}
