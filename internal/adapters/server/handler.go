package server

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"file-manager/internal/config"
	"file-manager/internal/domain"

	"github.com/sirupsen/logrus"
)

type Handler struct {
	uc            domain.FileManagement
	staticPath    string
	templateFile  string
	maxUploadSize int64
	forbiddenExt  []string
	messages      config.Messages
}

type browseData struct {
	Path   string
	Parent string
	Files  []domain.FileData
}

func NewHandler(
	uc domain.FileManagement,
	staticPath string,
	templateFile string,
	forbidden []string,
	maxUploadSize int64,
	messages config.Messages,
) *Handler {
	return &Handler{
		uc:            uc,
		staticPath:    staticPath,
		templateFile:  templateFile,
		maxUploadSize: maxUploadSize,
		forbiddenExt:  forbidden,
		messages:      messages,
	}
}

func (h *Handler) Browse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get(QueryParamPath)

	files, err := h.uc.List(path)
	if err != nil {
		h.handleError(w, err, h.messages.CannotListDirectory)
		return
	}

	// поиск родительской директорий
	var parent string
	if path != domain.PathEmpty {
		parent = h.normalizePath(filepath.Dir(path))
	}

	h.renderTemplate(w, browseData{
		Path:   path,
		Parent: parent,
		Files:  files,
	})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, func() error {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

		// роверяем ContentLength, чтобы отклонить слишком большие загрузки
		// ContentLength может быть -1 при chunked-передаче, поэтому дополнительно проверяем header.Size.
		if r.ContentLength > h.maxUploadSize {
			return fmt.Errorf("file size %d exceeds maximum %d: %w",
				r.ContentLength, h.maxUploadSize, domain.ErrUnsupportedOperation)
		}

		file, header, err := r.FormFile(FormParamFile)
		if err != nil {
			return fmt.Errorf("failed to get form file: %w", err)
		}
		defer file.Close()

		// дополнительная проверка размера, после разбора формы
		if header.Size > h.maxUploadSize {
			return fmt.Errorf("file size %d exceeds maximum %d: %w",
				header.Size, h.maxUploadSize, domain.ErrUnsupportedOperation)
		}

		if h.isForbidden(header.Filename) {
			return domain.ErrUnsupportedOperation
		}

		currentPath := r.FormValue(FormParamPath)
		targetPath := h.buildFullPath(currentPath, header.Filename)

		if uploadErr := h.uc.UploadFile(targetPath, file); uploadErr != nil {
			return uploadErr
		}

		logrus.WithFields(logrus.Fields{
			"operation": OperationUpload,
			"path":      targetPath,
			"size":      header.Size,
		}).Info(LogFileUploaded)

		h.redirectToPath(w, r, currentPath)
		return nil
	}, h.messages.InternalError)
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, func() error {
		name := r.FormValue(FormParamName)
		currentPath := r.FormValue(FormParamPath)
		fullPath := h.buildFullPath(currentPath, name)

		if err := h.uc.CreateFolder(fullPath); err != nil {
			return err
		}

		logrus.WithFields(logrus.Fields{
			"operation": OperationCreateFolder,
			"path":      fullPath,
		}).Info(LogFolderCreated)

		h.redirectToPath(w, r, currentPath)
		return nil
	}, h.messages.InternalError)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	path := h.getPathFromQuery(r)
	if err := h.uc.Delete(path); err != nil {
		h.handleError(w, err, h.messages.CannotDelete)
		return
	}

	logrus.WithFields(logrus.Fields{
		"operation": OperationDelete,
		"path":      path,
	}).Info(LogFileOrFolderDeleted)

	h.redirectToPath(w, r, h.normalizeParentPath(path))
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, func() error {
		oldPath := r.FormValue(FormParamOld)
		newName := r.FormValue(FormParamNew)

		parentPath := h.normalizeParentPath(oldPath)
		newFullPath := filepath.Join(parentPath, newName)
		if err := h.uc.Rename(oldPath, newFullPath); err != nil {
			return err
		}

		logrus.WithFields(logrus.Fields{
			"operation": OperationRename,
			"old_path":  oldPath,
			"new_path":  newFullPath,
		}).Info(LogFileOrFolderRenamed)

		h.redirectToPath(w, r, parentPath)
		return nil
	}, h.messages.InternalError)
}

func (h *Handler) getPathFromQuery(r *http.Request) string {
	return r.URL.Query().Get(QueryParamPath)
}

func (h *Handler) normalizeParentPath(path string) string {
	parent := filepath.Dir(path)
	if parent == domain.PathCurrent {
		return domain.PathEmpty
	}
	return parent
}

func (h *Handler) buildFullPath(currentPath, name string) string {
	if currentPath != domain.PathEmpty {
		return filepath.Join(currentPath, name)
	}
	return name
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, path string, isFolder bool) {
	if h.isForbidden(filepath.Base(path)) {
		http.Error(w, h.messages.ForbiddenFile, http.StatusForbidden)
		return
	}

	var err error
	if isFolder {
		err = h.uc.ServeFolderAsZip(w, path)
	} else {
		err = h.uc.ServeFile(w, r, path)
	}

	if err != nil {
		h.handleError(w, err, h.messages.CannotServe)
	}
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, h.getPathFromQuery(r), false)
}

func (h *Handler) DownloadFolder(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, h.getPathFromQuery(r), true)
}

func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request, handler func() error, message string) {
	if r.Method != http.MethodPost {
		h.redirectToPath(w, r, "")
		return
	}

	if err := handler(); err != nil {
		h.handleError(w, err, message)
		return
	}
}

type errorType int

const (
	errorTypeBadRequest errorType = iota
	errorTypeForbidden
	errorTypeNotFound
	errorTypeInternal
)

// getErrorType сопоставляет доменные ошибки с HTTP-кодами статуса.
// централизация преоброзования ошибок.
func (h *Handler) getErrorType(err error) errorType {
	switch {
	case errors.Is(err, domain.ErrPathTraversal) || errors.Is(err, domain.ErrInvalidName) || errors.Is(err, domain.ErrPathTooLong):
		return errorTypeBadRequest
	case errors.Is(err, domain.ErrUnsupportedOperation) || errors.Is(err, domain.ErrPermissionDenied):
		return errorTypeForbidden
	case errors.Is(err, domain.ErrFileNotFound):
		return errorTypeNotFound
	default:
		return errorTypeInternal
	}
}

func (h *Handler) handleError(w http.ResponseWriter, err error, message string) {
	var httpStatus int
	var clientMessage string

	switch h.getErrorType(err) {
	case errorTypeBadRequest:
		httpStatus = http.StatusBadRequest
		clientMessage = h.messages.InternalError
	case errorTypeForbidden:
		httpStatus = http.StatusForbidden
		clientMessage = h.messages.ForbiddenFile
	case errorTypeNotFound:
		httpStatus = http.StatusNotFound
		clientMessage = h.messages.InternalError
	case errorTypeInternal:
		httpStatus = http.StatusInternalServerError
		clientMessage = message
	}

	logrus.Errorf("HTTP %d Error: %s. Details: %+v", httpStatus, clientMessage, err)
	http.Error(w, clientMessage, httpStatus)
}

func (h *Handler) redirectToPath(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, RedirectPathTemplate+h.normalizePath(path), http.StatusFound)
}

func (h *Handler) normalizePath(path string) string {
	switch path {
	case domain.PathCurrent, domain.PathRoot:
		return domain.PathEmpty
	default:
		return path
	}
}

func (h *Handler) renderTemplate(w http.ResponseWriter, data browseData) {
	tmpl, parseErr := template.ParseFiles(filepath.Join(h.staticPath, h.templateFile))
	if parseErr != nil {
		logrus.Infoln(parseErr)
		http.Error(w, h.messages.TemplateError, http.StatusInternalServerError)
		return
	}

	if executeErr := tmpl.Execute(w, data); executeErr != nil {
		logrus.Infoln(executeErr)
		http.Error(w, h.messages.RenderError, http.StatusInternalServerError)
	}
}

// isForbidden проверяет расшрения файла, можно дальше масштабировать.
func (h *Handler) isForbidden(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	for _, forbidden := range h.forbiddenExt {
		if ext == forbidden || strings.HasPrefix(fileName, forbidden) {
			return true
		}
	}
	return false
}
