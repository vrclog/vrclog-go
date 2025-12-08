package vrclog

// compiledFilter holds pre-compiled filter configuration for efficient event filtering.
// It is created from FilterConfig during watcher/parser initialization.
type compiledFilter struct {
	include map[EventType]struct{}
	exclude map[EventType]struct{}
}

// newCompiledFilter creates a new compiledFilter from include and exclude slices.
// Returns nil if both slices are empty (no filtering needed).
func newCompiledFilter(include, exclude []EventType) *compiledFilter {
	if len(include) == 0 && len(exclude) == 0 {
		return nil
	}

	f := &compiledFilter{}

	if len(include) > 0 {
		f.include = make(map[EventType]struct{}, len(include))
		for _, t := range include {
			f.include[t] = struct{}{}
		}
	}

	if len(exclude) > 0 {
		f.exclude = make(map[EventType]struct{}, len(exclude))
		for _, t := range exclude {
			f.exclude[t] = struct{}{}
		}
	}

	return f
}

// Allows returns true if the given event type passes the filter.
// If include is non-empty, only types in include are allowed.
// Types in exclude are always rejected (exclude takes precedence).
func (f *compiledFilter) Allows(t EventType) bool {
	if f == nil {
		return true
	}

	// Check include list first (if specified)
	if len(f.include) > 0 {
		if _, ok := f.include[t]; !ok {
			return false
		}
	}

	// Check exclude list (always takes precedence)
	if len(f.exclude) > 0 {
		if _, ok := f.exclude[t]; ok {
			return false
		}
	}

	return true
}
