package domain

// TicketCounts summarizes every ticket visible to a viewer, independent of pagination.
type TicketCounts struct {
	Open         int64 `json:"open"`
	InProgress   int64 `json:"inProgress"`
	Closed       int64 `json:"closed"`
	HighPriority int64 `json:"highPriority"`
}
