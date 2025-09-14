// Package file provides centralized error handling utilities for file operations
// to reduce duplication and ensure consistent error messaging across the qf application.
package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileError represents different types of file operation errors
type FileError struct {
	Operation string // The operation being performed (e.g., "open", "read", "stat")
	Path      string // The file path involved
	Err       error  // The underlying error
	Type      FileErrorType
}

// FileErrorType categorizes different types of file errors
type FileErrorType int

const (
	ErrorTypeNotFound FileErrorType = iota
	ErrorTypePermission
	ErrorTypeReadFailure
	ErrorTypeWriteFailure
	ErrorTypeInvalidPath
	ErrorTypeOther
)

// Error implements the error interface
func (fe *FileError) Error() string {
	switch fe.Type {
	case ErrorTypeNotFound:
		return fmt.Sprintf("file not found: %s", fe.Path)
	case ErrorTypePermission:
		return fmt.Sprintf("permission denied: %s", fe.Path)
	case ErrorTypeReadFailure:
		return fmt.Sprintf("failed to read file %s: %v", fe.Path, fe.Err)
	case ErrorTypeWriteFailure:
		return fmt.Sprintf("failed to write file %s: %v", fe.Path, fe.Err)
	case ErrorTypeInvalidPath:
		return fmt.Sprintf("invalid file path: %s", fe.Path)
	default:
		return fmt.Sprintf("file operation '%s' failed for %s: %v", fe.Operation, fe.Path, fe.Err)
	}
}

// IsNotFound checks if the error is a file not found error
func (fe *FileError) IsNotFound() bool {
	return fe.Type == ErrorTypeNotFound
}

// IsPermissionDenied checks if the error is a permission denied error
func (fe *FileError) IsPermissionDenied() bool {
	return fe.Type == ErrorTypePermission
}

// FileErrorHandler provides centralized file error handling
type FileErrorHandler struct{}

// NewFileErrorHandler creates a new file error handler
func NewFileErrorHandler() *FileErrorHandler {
	return &FileErrorHandler{}
}

// WrapError wraps a file operation error with additional context
func (feh *FileErrorHandler) WrapError(operation, path string, err error) *FileError {
	if err == nil {
		return nil
	}

	// Determine error type
	errorType := ErrorTypeOther
	if os.IsNotExist(err) {
		errorType = ErrorTypeNotFound
	} else if os.IsPermission(err) {
		errorType = ErrorTypePermission
	} else {
		// Try to categorize based on operation
		switch operation {
		case "read", "open":
			errorType = ErrorTypeReadFailure
		case "write", "create":
			errorType = ErrorTypeWriteFailure
		}
	}

	return &FileError{
		Operation: operation,
		Path:      path,
		Err:       err,
		Type:      errorType,
	}
}

// OpenFile safely opens a file with proper error handling
func (feh *FileErrorHandler) OpenFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, feh.WrapError("open", path, err)
	}
	return file, nil
}

// StatFile safely gets file info with proper error handling
func (feh *FileErrorHandler) StatFile(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, feh.WrapError("stat", path, err)
	}
	return info, nil
}

// CreateFile safely creates a file with proper error handling
func (feh *FileErrorHandler) CreateFile(path string) (*os.File, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, feh.WrapError("create_dir", dir, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, feh.WrapError("create", path, err)
	}
	return file, nil
}

// ValidatePath validates that a file path is safe and accessible
func (feh *FileErrorHandler) ValidatePath(path string) error {
	if path == "" {
		return &FileError{
			Operation: "validate",
			Path:      path,
			Type:      ErrorTypeInvalidPath,
			Err:       fmt.Errorf("empty path"),
		}
	}

	// Check if path is absolute or can be made absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &FileError{
			Operation: "validate",
			Path:      path,
			Type:      ErrorTypeInvalidPath,
			Err:       err,
		}
	}

	// Check if we can stat the file (or its parent directory)
	if _, err := os.Stat(absPath); err != nil {
		// If file doesn't exist, check if parent directory exists
		if os.IsNotExist(err) {
			parentDir := filepath.Dir(absPath)
			if _, dirErr := os.Stat(parentDir); dirErr != nil {
				return feh.WrapError("validate", parentDir, dirErr)
			}
		} else {
			return feh.WrapError("validate", absPath, err)
		}
	}

	return nil
}

// SafeCloseFile safely closes a file with error handling
func (feh *FileErrorHandler) SafeCloseFile(file *os.File, path string) error {
	if file == nil {
		return nil
	}

	if err := file.Close(); err != nil {
		return feh.WrapError("close", path, err)
	}
	return nil
}

// Global error handler instance
var DefaultErrorHandler = NewFileErrorHandler()

// Convenience functions using the global error handler

// OpenFileWithErrorHandling opens a file with centralized error handling
func OpenFileWithErrorHandling(path string) (*os.File, error) {
	return DefaultErrorHandler.OpenFile(path)
}

// StatFileWithErrorHandling stats a file with centralized error handling
func StatFileWithErrorHandling(path string) (os.FileInfo, error) {
	return DefaultErrorHandler.StatFile(path)
}

// CreateFileWithErrorHandling creates a file with centralized error handling
func CreateFileWithErrorHandling(path string) (*os.File, error) {
	return DefaultErrorHandler.CreateFile(path)
}

// ValidatePathWithErrorHandling validates a path with centralized error handling
func ValidatePathWithErrorHandling(path string) error {
	return DefaultErrorHandler.ValidatePath(path)
}

// SafeCloseWithErrorHandling safely closes a file with centralized error handling
func SafeCloseWithErrorHandling(file *os.File, path string) error {
	return DefaultErrorHandler.SafeCloseFile(file, path)
}

// IsFileAccessible checks if a file can be accessed for reading
func IsFileAccessible(path string) error {
	return ValidatePathWithErrorHandling(path)
}

// ExpandPath expands a path to its absolute form
func ExpandPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path // fallback to original path
	}
	return absPath
}
