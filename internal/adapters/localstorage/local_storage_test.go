package localstorage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalStorageService(t *testing.T) {
	basePath := "/test/storage"
	dirPerm := os.FileMode(0o755)

	service := NewLocalStorageService(basePath, dirPerm)

	assert.NotNil(t, service)
	assert.Equal(t, basePath, service.basePath)
	assert.Equal(t, dirPerm, service.dirPerm)
}

func TestLocalStorageService_GetAbsolutePath(t *testing.T) {
	service := NewLocalStorageService("/base", 0o755)

	tests := []struct {
		name     string
		relPath  string
		expected string
	}{
		{"empty path", "", "/base"},
		{"simple path", "file.txt", "/base/file.txt"},
		{"nested path", "dir/subdir/file.txt", "/base/dir/subdir/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.GetAbsolutePath(tt.relPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLocalStorageService_ReadDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0o644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0o755)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		entries, err := service.ReadDirectory("")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2) // 2 files

		names := make(map[string]bool)
		for _, entry := range entries {
			names[entry.Name()] = true
		}
		assert.True(t, names["file1.txt"])
		assert.True(t, names["file2.txt"])
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := service.ReadDirectory("nonexistent")
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestLocalStorageService_WriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	t.Run("success", func(t *testing.T) {
		testData := "test file content"
		reader := strings.NewReader(testData)

		err := service.WriteFile("test.txt", reader)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
		require.NoError(t, err)
		assert.Equal(t, testData, string(data))
	})

	t.Run("nested path", func(t *testing.T) {
		testData := "nested content"
		reader := strings.NewReader(testData)

		err := service.WriteFile("dir/subdir/file.txt", reader)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(tmpDir, "dir/subdir/file.txt"))
		require.NoError(t, err)
		assert.Equal(t, testData, string(data))
	})

	t.Run("large file", func(t *testing.T) {
		largeData := strings.Repeat("a", 1024*1024) // 1MB
		reader := strings.NewReader(largeData)

		err := service.WriteFile("large.txt", reader)
		require.NoError(t, err)

		info, err := os.Stat(filepath.Join(tmpDir, "large.txt"))
		require.NoError(t, err)
		assert.Equal(t, int64(1024*1024), info.Size())
	})
}

func TestLocalStorageService_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	t.Run("remove file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		require.NoError(t, err)

		err = service.Remove("test.txt")
		require.NoError(t, err)

		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove directory", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "testdir")
		err := os.MkdirAll(dirPath, 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		err = service.Remove("testdir")
		require.NoError(t, err)

		_, err = os.Stat(dirPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("remove nonexistent", func(t *testing.T) {
		err := service.Remove("nonexistent")
		assert.NoError(t, err)
	})
}

func TestLocalStorageService_Move(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	t.Run("success", func(t *testing.T) {
		oldPath := filepath.Join(tmpDir, "old.txt")
		err := os.WriteFile(oldPath, []byte("content"), 0o644)
		require.NoError(t, err)

		err = service.Move("old.txt", "new.txt")
		require.NoError(t, err)

		// Old file should not exist
		_, err = os.Stat(oldPath)
		assert.True(t, os.IsNotExist(err))

		// New file should exist
		newPath := filepath.Join(tmpDir, "new.txt")
		data, err := os.ReadFile(newPath)
		require.NoError(t, err)
		assert.Equal(t, "content", string(data))
	})

	t.Run("empty destination", func(t *testing.T) {
		err := service.Move("old.txt", "")
		assert.Error(t, err)
		assert.Equal(t, os.ErrInvalid, err)
	})

	t.Run("nonexistent source", func(t *testing.T) {
		err := service.Move("nonexistent.txt", "new.txt")
		assert.Error(t, err)
	})
}

func TestLocalStorageService_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	t.Run("success", func(t *testing.T) {
		err := service.CreateDirectory("newdir")
		require.NoError(t, err)

		dirPath := filepath.Join(tmpDir, "newdir")
		info, err := os.Stat(dirPath)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("nested directory", func(t *testing.T) {
		err := service.CreateDirectory("dir1/dir2/dir3")
		require.NoError(t, err)

		dirPath := filepath.Join(tmpDir, "dir1/dir2/dir3")
		info, err := os.Stat(dirPath)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("already exists", func(t *testing.T) {
		err := service.CreateDirectory("existing")
		require.NoError(t, err)

		// Creating again should not error
		err = service.CreateDirectory("existing")
		assert.NoError(t, err)
	})
}

func TestLocalStorageService_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewLocalStorageService(tmpDir, 0o755)

	err := service.CreateDirectory("testdir")
	require.NoError(t, err)

	content := "test content"
	err = service.WriteFile("testdir/file.txt", strings.NewReader(content))
	require.NoError(t, err)

	entries, err := service.ReadDirectory("testdir")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "file.txt", entries[0].Name())

	err = service.Move("testdir/file.txt", "testdir/renamed.txt")
	require.NoError(t, err)

	entries, err = service.ReadDirectory("testdir")
	require.NoError(t, err)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	assert.Contains(t, names, "renamed.txt")
	assert.NotContains(t, names, "file.txt")

	err = service.Remove("testdir")
	require.NoError(t, err)

	_, err = service.ReadDirectory("testdir")
	assert.Error(t, err)
}
