package domain

// TicketCounts summarizes every ticket visible to a viewer, independent of pagination.
type TicketCounts struct {
	All          int64 `json:"all"`
	Unassigned   int64 `json:"unassigned"`
	AssignedToMe int64 `json:"assignedToMe"`
	Open         int64 `json:"open"`
	InProgress   int64 `json:"inProgress"`
	Closed       int64 `json:"closed"`
	HighPriority int64 `json:"highPriority"`
}
