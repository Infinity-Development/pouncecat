package source

type Source interface {
	// Returns the records of a entity (collection in mongo, row in postgres etc)
	GetRecords(entity string) ([]map[string]any, error)
	// Gets the count of records in a entity
	GetCount(entity string) (int64, error)
	// Extra parsers
	ExtParse(res any) (any, error)
	// Fetches all table/collection names
	RecordList() ([]string, error)
}
