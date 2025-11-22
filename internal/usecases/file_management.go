package usecases

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"file-manager/internal/config"
	"file-manager/internal/domain"
)

type FileManagementUseCase struct {
	storage   domain.FileStorage
	cfg       *config.Config
	validName *regexp.Regexp
}

func NewFileManagementUseCase(storage domain.FileStorage, cfg *config.Config) *FileManagementUseCase {
	regex := regexp.MustCompile(cfg.File.ValidNameRegex)
	return &FileManagementUseCase{
		storage:   storage,
		cfg:       cfg,
		validName: regex,
	}
}

// (НА БУДУЩЕЕ, ЕСЛИ БУДУ НАСТРАИВАТЬ УДАЛЕННЫЙ ДОСТУП) sanitizePath нужен для нормализации путей, чтобы атаки через обход директорий.
func (uc *FileManagementUseCase) sanitizePath(path string) (string, error) {
	clean := filepath.Clean(path)

	// отклоняю абсолютные пути, чтобы предотвратить доступ за пределы базовой директории хранилища.
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths are not allowed: %w", domain.ErrPathTraversal)
	}

	// проверяю, что путь остаётся внутри базовой директории, проверяя, содержит ли
	// относительный путь от базы до разрешённого пути "..".
	basePath := filepath.Clean(uc.storage.GetAbsolutePath(""))
	fullPath := filepath.Join(basePath, clean)

	rel, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", domain.ErrPathTraversal)
	}

	if strings.HasPrefix(rel, domain.PathTraversalPrefix) {
		return "", fmt.Errorf("path traversal detected: %w", domain.ErrPathTraversal)
	}

	if len(clean) > uc.cfg.File.MaxNameLength {
		return "", fmt.Errorf("path '%s' too long (%d > %d): %w",
			path, len(clean), uc.cfg.File.MaxNameLength, domain.ErrPathTooLong)
	}

	// валидация имени файла, чтобы не было недопустимых символов
	base := filepath.Base(clean)
	if base != domain.PathCurrent && base != domain.PathEmpty && !uc.validName.MatchString(base) {
		return "", fmt.Errorf("base name '%s' is invalid: %w", base, domain.ErrInvalidName)
	}

	return clean, nil
}

func (uc *FileManagementUseCase) List(path string) ([]domain.FileData, error) {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := uc.storage.ReadDirectory(sanitizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("could not read directory '%s': %w", sanitizedPath, domain.ErrFileNotFound)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("could not read directory '%s': %w", sanitizedPath, domain.ErrPermissionDenied)
		}
		return nil, fmt.Errorf("failed to list path '%s': %w", sanitizedPath, err)
	}

	files := make([]domain.FileData, 0, len(entries))
	for _, fi := range entries {
		files = append(files, domain.FileData{
			Name:  fi.Name(),
			IsDir: fi.IsDir(),
		})
	}

	return files, nil
}

func (uc *FileManagementUseCase) UploadFile(path string, file io.Reader) error {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return err
	}
	if writeErr := uc.storage.WriteFile(sanitizedPath, file); writeErr != nil {
		return fmt.Errorf("failed to upload file to '%s': %w", sanitizedPath, writeErr)
	}
	return nil
}

func (uc *FileManagementUseCase) Delete(path string) error {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return err
	}
	if removeErr := uc.storage.Remove(sanitizedPath); removeErr != nil {
		return fmt.Errorf("could not delete file/folder '%s': %w", sanitizedPath, removeErr)
	}
	return nil
}

func (uc *FileManagementUseCase) Rename(oldPath, newPath string) error {
	sanitizedOldPath, err := uc.sanitizePath(oldPath)
	if err != nil {
		return err
	}
	sanitizedNewPath, err := uc.sanitizePath(newPath)
	if err != nil {
		return err
	}
	if moveErr := uc.storage.Move(sanitizedOldPath, sanitizedNewPath); moveErr != nil {
		return fmt.Errorf("could not rename '%s' to '%s': %w", sanitizedOldPath, sanitizedNewPath, moveErr)
	}
	return nil
}

func (uc *FileManagementUseCase) CreateFolder(path string) error {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return err
	}
	if createErr := uc.storage.CreateDirectory(sanitizedPath); createErr != nil {
		return fmt.Errorf("could not create folder '%s': %w", sanitizedPath, createErr)
	}
	return nil
}

func (uc *FileManagementUseCase) ServeFile(w http.ResponseWriter, r *http.Request, path string) error {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return err
	}

	fullPath := uc.storage.GetAbsolutePath(sanitizedPath)
	if _, statErr := os.Stat(fullPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("file not found at '%s': %w", sanitizedPath, domain.ErrFileNotFound)
		}
		return fmt.Errorf("failed to stat file at '%s': %w", sanitizedPath, statErr)
	}

	// MIME.
	// для корреткного скачивания файлов.
	mimeType := mime.TypeByExtension(filepath.Ext(fullPath))
	if mimeType == domain.PathEmpty {
		mimeType = domain.MIMEOctetStream
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))
	http.ServeFile(w, r, fullPath)
	return nil
}

// shouldSkipFile исключить чувствительные файлы из zip архива.
// чтобы не включить скрытые или системные файлы.
func (uc *FileManagementUseCase) shouldSkipFile(info os.FileInfo) bool {
	return strings.HasPrefix(info.Name(), domain.HiddenFilePrefix)
}

// добавление файлов в zip архив
func (uc *FileManagementUseCase) addFileToZip(zipWriter *zip.Writer, fullPath, filePath string) error {
	rel, err := filepath.Rel(fullPath, filePath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	dstFile, err := zipWriter.Create(rel)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	srcFile, openErr := os.Open(filePath)
	if openErr != nil {
		return fmt.Errorf("failed to open file: %w", openErr)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			logrus.Warnf("Failed to close file %s: %v", filePath, closeErr)
		}
	}()

	if _, copyErr := io.Copy(dstFile, srcFile); copyErr != nil {
		return fmt.Errorf("failed to copy file to zip: %w", copyErr)
	}

	return nil
}

// createZipArchive рекурсивно обхожу дерево директорий и добавляю все не скрытые файлы
func (uc *FileManagementUseCase) createZipArchive(zipWriter *zip.Writer, fullPath string) error {
	return filepath.Walk(fullPath, func(file string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if uc.shouldSkipFile(info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		return uc.addFileToZip(zipWriter, fullPath, file)
	})
}

func (uc *FileManagementUseCase) ServeFolderAsZip(w http.ResponseWriter, path string) error {
	sanitizedPath, err := uc.sanitizePath(path)
	if err != nil {
		return err
	}

	fullPath := uc.storage.GetAbsolutePath(sanitizedPath)
	info, statErr := os.Stat(fullPath)
	if statErr != nil || !info.IsDir() {
		return fmt.Errorf("could not stat folder '%s': %w", sanitizedPath, domain.ErrFileNotFound)
	}

	zipName := filepath.Base(sanitizedPath) + domain.ExtensionZip
	w.Header().Set("Content-Type", domain.MIMEZip)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipName))

	zipWriter := zip.NewWriter(w)
	defer func() {
		if closeErr := zipWriter.Close(); closeErr != nil {
			logrus.Errorf("Failed to close zip writer: %v", closeErr)
		}
	}()

	if archiveErr := uc.createZipArchive(zipWriter, fullPath); archiveErr != nil {
		return fmt.Errorf("failed to create zip for folder '%s': %w", sanitizedPath, archiveErr)
	}

	return nil
}
