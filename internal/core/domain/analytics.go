package domain

import (
	"time"

	"github.com/google/uuid"
)

type StatusCount struct {
	Status TicketStatus
	Count  int64
}

type WorkloadItem struct {
	AssigneeID *uuid.UUID
	FullName   string
	Email      string
	Count      int64
}

type VolumePoint struct {
	Day           time.Time
	CreatedCount  int64
	ResolvedCount int64
}

type AnalyticsOverview struct {
	StatusCounts []StatusCount
	Workload     []WorkloadItem
	Volume       []VolumePoint
	MTTRHours    float64
}
