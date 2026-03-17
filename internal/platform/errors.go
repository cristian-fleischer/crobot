package platform

import "errors"

// Sentinel errors for validation and factory operations.
var (
	// ErrEmptyPath indicates a ReviewFinding has an empty Path field.
	ErrEmptyPath = errors.New("path must not be empty")

	// ErrInvalidLine indicates a ReviewFinding has a non-positive Line value.
	ErrInvalidLine = errors.New("line must be greater than 0")

	// ErrInvalidSide indicates a ReviewFinding has a Side value that is
	// neither "new" nor "old".
	ErrInvalidSide = errors.New("side must be \"new\" or \"old\"")

	// ErrInvalidSeverity indicates a ReviewFinding has a Severity value that
	// is not one of "info", "warning", or "error".
	ErrInvalidSeverity = errors.New("severity must be \"info\", \"warning\", or \"error\"")

	// ErrEmptyCategory indicates a ReviewFinding has an empty Category field.
	ErrEmptyCategory = errors.New("category must not be empty")

	// ErrEmptyMessage indicates a ReviewFinding has an empty Message field.
	ErrEmptyMessage = errors.New("message must not be empty")

	// ErrInvalidSeverityScore indicates a ReviewFinding has a SeverityScore
	// outside the valid range of 1–10.
	ErrInvalidSeverityScore = errors.New("severity_score must be between 1 and 10 (or 0 to omit)")

	// ErrUnknownPlatform indicates the factory was asked for a platform name
	// it does not recognise.
	ErrUnknownPlatform = errors.New("unknown platform")
)
