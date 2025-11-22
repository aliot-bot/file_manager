package server

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"file-manager/internal/config"
	"file-manager/internal/domain"
)

type mockFileManagement struct {
	listFunc             func(path string) ([]domain.FileData, error)
	uploadFileFunc       func(path string, file io.Reader) error
	createFolderFunc     func(path string) error
	deleteFunc           func(path string) error
	renameFunc           func(oldPath, newPath string) error
	serveFileFunc        func(w http.ResponseWriter, r *http.Request, path string) error
	serveFolderAsZipFunc func(w http.ResponseWriter, path string) error
}

func (m *mockFileManagement) List(path string) ([]domain.FileData, error) {
	if m.listFunc != nil {
		return m.listFunc(path)
	}
	return nil, nil
}

func (m *mockFileManagement) UploadFile(path string, file io.Reader) error {
	if m.uploadFileFunc != nil {
		return m.uploadFileFunc(path, file)
	}
	return nil
}

func (m *mockFileManagement) CreateFolder(path string) error {
	if m.createFolderFunc != nil {
		return m.createFolderFunc(path)
	}
	return nil
}

func (m *mockFileManagement) Delete(path string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(path)
	}
	return nil
}

func (m *mockFileManagement) Rename(oldPath, newPath string) error {
	if m.renameFunc != nil {
		return m.renameFunc(oldPath, newPath)
	}
	return nil
}

func (m *mockFileManagement) ServeFile(w http.ResponseWriter, r *http.Request, path string) error {
	if m.serveFileFunc != nil {
		return m.serveFileFunc(w, r, path)
	}
	return nil
}

func (m *mockFileManagement) ServeFolderAsZip(w http.ResponseWriter, path string) error {
	if m.serveFolderAsZipFunc != nil {
		return m.serveFolderAsZipFunc(w, path)
	}
	return nil
}

func TestNewHandler(t *testing.T) {
	mockUC := &mockFileManagement{}
	messages := config.Messages{
		CannotListDirectory: "Cannot list",
		InternalError:       "Internal error",
	}

	handler := NewHandler(
		mockUC,
		"/static",
		"index.html",
		[]string{".env"},
		1024*1024,
		messages,
	)

	assert.NotNil(t, handler)
	assert.Equal(t, mockUC, handler.uc)
	assert.Equal(t, "/static", handler.staticPath)
	assert.Equal(t, "index.html", handler.templateFile)
	assert.Equal(t, int64(1024*1024), handler.maxUploadSize)
	assert.Equal(t, []string{".env"}, handler.forbiddenExt)
	assert.Equal(t, messages, handler.messages)
}

func TestHandler_Browse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		templateFile := filepath.Join(tmpDir, "index.html")
		err := os.WriteFile(templateFile, []byte("<html>{{.Path}}</html>"), 0o644)
		require.NoError(t, err)

		mockUC := &mockFileManagement{
			listFunc: func(path string) ([]domain.FileData, error) {
				return []domain.FileData{
					{Name: "file1.txt", IsDir: false},
					{Name: "dir1", IsDir: true},
				}, nil
			},
		}
		handler := NewHandler(
			mockUC,
			tmpDir,
			"index.html",
			[]string{".env"},
			1024*1024,
			config.Messages{
				CannotListDirectory: "Cannot list",
				TemplateError:       "Template error",
				RenderError:         "Render error",
				ForbiddenFile:       "Forbidden",
				CannotServe:         "Cannot serve",
				CannotDelete:        "Cannot delete",
				InternalError:       "Internal error",
			},
		)

		req := httptest.NewRequest("GET", "/?path=test", nil)
		w := httptest.NewRecorder()

		handler.Browse(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("error listing", func(t *testing.T) {
		mockUC := &mockFileManagement{
			listFunc: func(path string) ([]domain.FileData, error) {
				return nil, domain.ErrFileNotFound
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("GET", "/?path=nonexistent", nil)
		w := httptest.NewRecorder()

		handler.Browse(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_Upload(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var uploadedPath string
		mockUC := &mockFileManagement{
			uploadFileFunc: func(path string, file io.Reader) error {
				uploadedPath = path
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		var buf bytes.Buffer
		writer := multipartWriter(t, &buf, "test.txt", "test content", "")
		req := httptest.NewRequest("POST", "/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.Upload(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Contains(t, uploadedPath, "test.txt")
	})

	t.Run("forbidden extension", func(t *testing.T) {
		handler := createTestHandler(&mockFileManagement{})
		handler.forbiddenExt = []string{".env"}

		var buf bytes.Buffer
		writer := multipartWriter(t, &buf, "config.env", "secret", "")
		req := httptest.NewRequest("POST", "/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.Upload(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("file too large", func(t *testing.T) {
		handler := createTestHandler(&mockFileManagement{})
		handler.maxUploadSize = 10

		var buf bytes.Buffer
		writer := multipartWriter(t, &buf, "large.txt", strings.Repeat("a", 100), "")
		req := httptest.NewRequest("POST", "/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		handler.Upload(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("wrong method", func(t *testing.T) {
		handler := createTestHandler(&mockFileManagement{})

		req := httptest.NewRequest("GET", "/upload", nil)
		w := httptest.NewRecorder()

		handler.Upload(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
	})
}

func TestHandler_CreateFolder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var createdPath string
		mockUC := &mockFileManagement{
			createFolderFunc: func(path string) error {
				createdPath = path
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("POST", "/create-folder", strings.NewReader("name=newfolder&path="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.CreateFolder(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Contains(t, createdPath, "newfolder")
	})

	t.Run("error creating", func(t *testing.T) {
		mockUC := &mockFileManagement{
			createFolderFunc: func(path string) error {
				return domain.ErrInvalidName
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("POST", "/create-folder", strings.NewReader("name=invalid<>name&path="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.CreateFolder(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var deletedPath string
		mockUC := &mockFileManagement{
			deleteFunc: func(path string) error {
				deletedPath = path
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("GET", "/delete?path=test.txt", nil)
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "test.txt", deletedPath)
	})

	t.Run("error deleting", func(t *testing.T) {
		mockUC := &mockFileManagement{
			deleteFunc: func(path string) error {
				return domain.ErrFileNotFound
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("GET", "/delete?path=nonexistent", nil)
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_Rename(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var oldPath, newPath string
		mockUC := &mockFileManagement{
			renameFunc: func(old, new string) error {
				oldPath = old
				newPath = new
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("POST", "/rename", strings.NewReader("old=old.txt&new=new.txt"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.Rename(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "old.txt", oldPath)
		assert.Contains(t, newPath, "new.txt")
	})
}

func TestHandler_Download(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockUC := &mockFileManagement{
			serveFileFunc: func(w http.ResponseWriter, r *http.Request, path string) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("file content"))
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("GET", "/download?path=test.txt", nil)
		w := httptest.NewRecorder()

		handler.Download(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "file content")
	})
}

func TestHandler_DownloadFolder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockUC := &mockFileManagement{
			serveFolderAsZipFunc: func(w http.ResponseWriter, path string) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("zip content"))
				return nil
			},
		}
		handler := createTestHandler(mockUC)

		req := httptest.NewRequest("GET", "/download-folder?path=testdir", nil)
		w := httptest.NewRecorder()

		handler.DownloadFolder(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "zip content")
	})
}

func TestHandler_isForbidden(t *testing.T) {
	handler := createTestHandler(&mockFileManagement{})
	handler.forbiddenExt = []string{".env", ".gitignore"}

	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{"forbidden extension", "config.env", true},
		{"forbidden extension case insensitive", "CONFIG.ENV", true},
		{"forbidden prefix", ".gitignore", true},
		{"allowed file", "test.txt", false},
		{"allowed extension", "script.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isForbidden(tt.fileName)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestHandler_getErrorType(t *testing.T) {
	handler := createTestHandler(&mockFileManagement{})

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"path traversal", domain.ErrPathTraversal, http.StatusBadRequest},
		{"invalid name", domain.ErrInvalidName, http.StatusBadRequest},
		{"path too long", domain.ErrPathTooLong, http.StatusBadRequest},
		{"unsupported operation", domain.ErrUnsupportedOperation, http.StatusForbidden},
		{"permission denied", domain.ErrPermissionDenied, http.StatusForbidden},
		{"file not found", domain.ErrFileNotFound, http.StatusNotFound},
		{"unknown error", errors.New("unknown"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorType := handler.getErrorType(tt.err)
			var status int
			switch errorType {
			case errorTypeBadRequest:
				status = http.StatusBadRequest
			case errorTypeForbidden:
				status = http.StatusForbidden
			case errorTypeNotFound:
				status = http.StatusNotFound
			case errorTypeInternal:
				status = http.StatusInternalServerError
			}
			assert.Equal(t, tt.want, status)
		})
	}
}

func createTestHandler(uc domain.FileManagement) *Handler {
	return NewHandler(
		uc,
		"/static",
		"index.html",
		[]string{".env"},
		1024*1024,
		config.Messages{
			CannotListDirectory: "Cannot list",
			TemplateError:       "Template error",
			RenderError:         "Render error",
			ForbiddenFile:       "Forbidden",
			CannotServe:         "Cannot serve",
			CannotDelete:        "Cannot delete",
			InternalError:       "Internal error",
		},
	)
}

func multipartWriter(t *testing.T, buf *bytes.Buffer, filename, content, path string) *multipart.Writer {
	writer := multipart.NewWriter(buf)

	fileWriter, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fileWriter.Write([]byte(content))
	require.NoError(t, err)

	if path != "" {
		err = writer.WriteField("path", path)
		require.NoError(t, err)
	}

	err = writer.Close()
	require.NoError(t, err)

	return writer
}
