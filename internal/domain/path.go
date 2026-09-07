package domain

import "strings"

// LogicalPath is an immutable absolute path using '/' independently of the host OS.
type LogicalPath struct {
	value string
}

func ParseLogicalPath(value string) (LogicalPath, error) {
	if value == "/" {
		return LogicalPath{value: value}, nil
	}
	if !strings.HasPrefix(value, "/") {
		return LogicalPath{}, invalid("path", ErrInvalidPath)
	}

	rawSegments := strings.Split(value[1:], "/")
	segments := make([]Name, len(rawSegments))
	for index, rawSegment := range rawSegments {
		name, err := NewName(rawSegment)
		if err != nil || rawSegment == "." || rawSegment == ".." {
			return LogicalPath{}, invalid("path", ErrInvalidPath)
		}
		segments[index] = name
	}
	return NewLogicalPath(segments...)
}

// NewLogicalPath constructs the canonical path for validated segments.
// No segments represents the root path.
func NewLogicalPath(segments ...Name) (LogicalPath, error) {
	if len(segments) == 0 {
		return LogicalPath{value: "/"}, nil
	}

	values := make([]string, len(segments))
	for index, segment := range segments {
		value := segment.String()
		if _, err := NewName(value); err != nil || value == "." || value == ".." {
			return LogicalPath{}, invalid("path", ErrInvalidPath)
		}
		values[index] = value
	}
	return LogicalPath{value: "/" + strings.Join(values, "/")}, nil
}

func (path LogicalPath) String() string {
	return path.value
}

func (path LogicalPath) IsRoot() bool {
	return path.value == "/"
}

// Segments returns a copy of the path's names.
func (path LogicalPath) Segments() []Name {
	if path.IsRoot() || path.value == "" {
		return nil
	}

	rawSegments := strings.Split(path.value[1:], "/")
	segments := make([]Name, len(rawSegments))
	for index, value := range rawSegments {
		segments[index] = Name{value: value}
	}
	return segments
}

func (path LogicalPath) MarshalText() ([]byte, error) {
	if path.value == "" {
		return nil, invalid("path", ErrInvalidPath)
	}
	return []byte(path.value), nil
}
