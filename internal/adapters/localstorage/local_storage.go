package localstorage

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type LocalStorageService struct {
	basePath string
	dirPerm  os.FileMode
}

func NewLocalStorageService(basePath string, dirPerm os.FileMode) *LocalStorageService {
	return &LocalStorageService{
		basePath: basePath,
		dirPerm:  dirPerm,
	}
}

func (s *LocalStorageService) GetAbsolutePath(relPath string) string {
	return filepath.Join(s.basePath, relPath)
}

func (s *LocalStorageService) ReadDirectory(relPath string) ([]os.FileInfo, error) {
	fullPath := s.GetAbsolutePath(relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	// О - оптимизация.
	files := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, infoErr := e.Info()
		if infoErr != nil {
			// пропуск файл, например, с битыми симлинками.
			logrus.Warnf("Failed to get info for %s: %v", e.Name(), infoErr)
			continue
		}
		files = append(files, info)
	}

	return files, nil
}

// WriteFile записывает файл в хранилище.
// директории с нужными правами
func (s *LocalStorageService) WriteFile(relPath string, file io.Reader) error {
	fullPath := s.GetAbsolutePath(relPath)
	dir := filepath.Dir(fullPath)

	// тут я не знаю на самом деле какая практика будет лучше, но сделал так:
	// создаем родительские директории, если они отсутствуют, чтобы поддерживать вложенные пути.
	if err := os.MkdirAll(dir, s.dirPerm); err != nil {
		return err
	}

	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			logrus.Warnf("Failed to close file %s: %v", fullPath, closeErr)
		}
	}()

	_, err = io.Copy(out, file)
	return err
}

func (s *LocalStorageService) Remove(relPath string) error {
	return os.RemoveAll(s.GetAbsolutePath(relPath))
}

// Move переименовывает файл или директорий внутри базового хранилища.
// пустой путь отклоняется, чтобы избежать случайную потерю данных.
func (s *LocalStorageService) Move(oldRel, newRel string) error {
	if newRel == "" {
		return os.ErrInvalid
	}
	return os.Rename(s.GetAbsolutePath(oldRel), s.GetAbsolutePath(newRel))
}

func (s *LocalStorageService) CreateDirectory(relPath string) error {
	return os.MkdirAll(s.GetAbsolutePath(relPath), s.dirPerm)
}
