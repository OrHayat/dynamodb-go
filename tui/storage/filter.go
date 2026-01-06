package storage

// FilterState holds the filter configuration for a table
type FilterState struct {
	Field    string
	Type     int // index into filterTypes (0=String, 1=Number, 2=Boolean)
	Operator int // index into filterOperators
	Value    string
}

// FilterStorage interface for persisting filters per table
type FilterStorage interface {
	Get(tableName string) (FilterState, bool)
	Set(tableName string, state FilterState)
	Clear(tableName string)
}

// MemoryFilterStorage implements FilterStorage with in-memory storage
type MemoryFilterStorage struct {
	filters map[string]FilterState
}

// NewMemoryFilterStorage creates a new in-memory filter storage
func NewMemoryFilterStorage() *MemoryFilterStorage {
	return &MemoryFilterStorage{
		filters: make(map[string]FilterState),
	}
}

// Get retrieves the filter state for a table
func (s *MemoryFilterStorage) Get(tableName string) (FilterState, bool) {
	state, ok := s.filters[tableName]
	return state, ok
}

// Set stores the filter state for a table
func (s *MemoryFilterStorage) Set(tableName string, state FilterState) {
	s.filters[tableName] = state
}

// Clear removes the filter state for a table
func (s *MemoryFilterStorage) Clear(tableName string) {
	delete(s.filters, tableName)
}
