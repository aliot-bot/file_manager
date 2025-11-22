package domain

import "errors"

var (
	ErrPathTraversal        = errors.New("path traversal is not allowed")
	ErrPathTooLong          = errors.New("path too long")
	ErrInvalidName          = errors.New("invalid file or folder name")
	ErrFileNotFound         = errors.New("file or folder not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrUnsupportedOperation = errors.New("unsupported operation")
)
