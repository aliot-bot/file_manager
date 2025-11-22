package usecases

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"file-manager/internal/config"
	"file-manager/internal/domain"
)

// mockFileStorage is a mock implementation of FileStorage for testing.
type mockFileStorage struct {
	basePath string

	readDirectoryFunc   func(relPath string) ([]os.FileInfo, error)
	writeFileFunc       func(relPath string, file io.Reader) error
	removeFunc          func(relPath string) error
	moveFunc            func(oldRel, newRel string) error
	createDirectoryFunc func(relPath string) error
	getAbsolutePathFunc func(relPath string) string
}

func (m *mockFileStorage) ReadDirectory(relPath string) ([]os.FileInfo, error) {
	if m.readDirectoryFunc != nil {
		return m.readDirectoryFunc(relPath)
	}
	return nil, nil
}

func (m *mockFileStorage) WriteFile(relPath string, file io.Reader) error {
	if m.writeFileFunc != nil {
		return m.writeFileFunc(relPath, file)
	}
	return nil
}

func (m *mockFileStorage) Remove(relPath string) error {
	if m.removeFunc != nil {
		return m.removeFunc(relPath)
	}
	return nil
}

func (m *mockFileStorage) Move(oldRel, newRel string) error {
	if m.moveFunc != nil {
		return m.moveFunc(oldRel, newRel)
	}
	return nil
}

func (m *mockFileStorage) CreateDirectory(relPath string) error {
	if m.createDirectoryFunc != nil {
		return m.createDirectoryFunc(relPath)
	}
	return nil
}

func (m *mockFileStorage) GetAbsolutePath(relPath string) string {
	if m.getAbsolutePathFunc != nil {
		return m.getAbsolutePathFunc(relPath)
	}
	return filepath.Join(m.basePath, relPath)
}

func TestNewFileManagementUseCase(t *testing.T) {
	cfg := &config.Config{
		File: config.FileConfig{
			ValidNameRegex: `^[\w\-. ]+$`,
		},
	}
	mockStorage := &mockFileStorage{basePath: "/test"}

	uc := NewFileManagementUseCase(mockStorage, cfg)

	assert.NotNil(t, uc)
	assert.Equal(t, mockStorage, uc.storage)
	assert.Equal(t, cfg, uc.cfg)
	assert.NotNil(t, uc.validName)
}

func TestFileManagementUseCase_sanitizePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		basePath    string
		maxLength   int
		validRegex  string
		wantErr     error
		description string
	}{
		{
			name:        "valid relative path",
			path:        "documents/file.txt",
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     nil,
			description: "should accept valid relative path",
		},
		{
			name:        "absolute path rejected",
			path:        "/absolute/path",
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     domain.ErrPathTraversal,
			description: "should reject absolute paths",
		},
		{
			name:        "path traversal detected",
			path:        "../../etc/passwd",
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     domain.ErrPathTraversal,
			description: "should detect path traversal attempts",
		},
		{
			name:        "path too long",
			path:        strings.Repeat("a", 300),
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     domain.ErrPathTooLong,
			description: "should reject paths exceeding max length",
		},
		{
			name:        "invalid characters",
			path:        "file<script>.txt",
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     domain.ErrInvalidName,
			description: "should reject invalid characters",
		},
		{
			name:        "empty path",
			path:        "",
			basePath:    "/storage",
			maxLength:   255,
			validRegex:  `^[\w\-. ]+$`,
			wantErr:     nil,
			description: "should accept empty path for root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				File: config.FileConfig{
					MaxNameLength:  tt.maxLength,
					ValidNameRegex: tt.validRegex,
				},
			}
			mockStorage := &mockFileStorage{
				basePath: tt.basePath,
				getAbsolutePathFunc: func(relPath string) string {
					return filepath.Join(tt.basePath, relPath)
				},
			}
			uc := NewFileManagementUseCase(mockStorage, cfg)

			result, err := uc.sanitizePath(tt.path)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected error %v, got %v", tt.wantErr, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestFileManagementUseCase_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		mockInfo := &mockFileInfo{name: "test.txt", isDir: false}
		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			readDirectoryFunc: func(relPath string) ([]os.FileInfo, error) {
				return []os.FileInfo{mockInfo}, nil
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		files, err := uc.List("")

		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, "test.txt", files[0].Name)
		assert.False(t, files[0].IsDir)
	})

	t.Run("directory not found", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			readDirectoryFunc: func(relPath string) ([]os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		files, err := uc.List("nonexistent")

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrFileNotFound))
		assert.Nil(t, files)
	})

	t.Run("permission denied", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			readDirectoryFunc: func(relPath string) ([]os.FileInfo, error) {
				return nil, os.ErrPermission
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		files, err := uc.List("restricted")

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrPermissionDenied))
		assert.Nil(t, files)
	})
}

func TestFileManagementUseCase_UploadFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		var writtenPath string
		var writtenData []byte
		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			writeFileFunc: func(relPath string, file io.Reader) error {
				writtenPath = relPath
				writtenData, _ = io.ReadAll(file)
				return nil
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		testData := strings.NewReader("test content")
		err := uc.UploadFile("test.txt", testData)

		assert.NoError(t, err)
		assert.Equal(t, "test.txt", writtenPath)
		assert.Equal(t, "test content", string(writtenData))
	})

	t.Run("invalid path", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		err := uc.UploadFile("../../etc/passwd", strings.NewReader("evil"))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrPathTraversal))
	})
}

func TestFileManagementUseCase_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		var deletedPath string
		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			removeFunc: func(relPath string) error {
				deletedPath = relPath
				return nil
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		err := uc.Delete("test.txt")

		assert.NoError(t, err)
		assert.Equal(t, "test.txt", deletedPath)
	})
}

func TestFileManagementUseCase_Rename(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		var oldPath, newPath string
		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			moveFunc: func(oldRel, newRel string) error {
				oldPath = oldRel
				newPath = newRel
				return nil
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		err := uc.Rename("old.txt", "new.txt")

		assert.NoError(t, err)
		assert.Equal(t, "old.txt", oldPath)
		assert.Equal(t, "new.txt", newPath)
	})
}

func TestFileManagementUseCase_CreateFolder(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := &config.Config{
			File: config.FileConfig{
				MaxNameLength:  255,
				ValidNameRegex: `^[\w\-. ]+$`,
			},
		}

		var createdPath string
		mockStorage := &mockFileStorage{
			basePath: "/storage",
			getAbsolutePathFunc: func(relPath string) string {
				return "/storage"
			},
			createDirectoryFunc: func(relPath string) error {
				createdPath = relPath
				return nil
			},
		}
		uc := NewFileManagementUseCase(mockStorage, cfg)

		err := uc.CreateFolder("newfolder")

		assert.NoError(t, err)
		assert.Equal(t, "newfolder", createdPath)
	})
}

func TestFileManagementUseCase_shouldSkipFile(t *testing.T) {
	cfg := &config.Config{
		File: config.FileConfig{
			ValidNameRegex: `^[\w\-. ]+$`,
		},
	}
	uc := NewFileManagementUseCase(&mockFileStorage{}, cfg)

	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{"hidden file", ".hidden", true},
		{"normal file", "normal.txt", false},
		{"hidden directory", ".git", true},
		{"normal directory", "docs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &mockFileInfo{name: tt.fileName}
			result := uc.shouldSkipFile(info)
			assert.Equal(t, tt.want, result)
		})
	}
}

type mockFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) Sys() interface{}   { return nil }
