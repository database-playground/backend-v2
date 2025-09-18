package models

type SQLExecutionResult struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

type UserSQLExecutionResult struct {
	SQLExecutionResult

	// MatchAnswer is true if the user's answer matches the reference answer
	MatchAnswer bool `json:"match_answer"`
}
