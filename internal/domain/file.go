package domain

import (
	"io"
	"net/http"
	"os"
)

// FileData информация о файле или директории.
type FileData struct {
	Name  string
	IsDir bool
}

// FileStorage для операций работы с файловым хранилищем.
type FileStorage interface {
	ReadDirectory(relPath string) ([]os.FileInfo, error)
	WriteFile(relPath string, file io.Reader) error
	Remove(relPath string) error
	Move(oldRel, newRel string) error
	CreateDirectory(relPath string) error
	GetAbsolutePath(relPath string) string
}

// FileManagement для сценариев управления файлами.
type FileManagement interface {
	List(path string) ([]FileData, error)
	UploadFile(path string, file io.Reader) error
	CreateFolder(path string) error
	Delete(path string) error
	Rename(oldPath, newPath string) error
	ServeFile(w http.ResponseWriter, r *http.Request, path string) error
	ServeFolderAsZip(w http.ResponseWriter, path string) error
}
