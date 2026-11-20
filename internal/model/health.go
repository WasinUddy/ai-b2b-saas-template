package model

// HealthStatus is the liveness or readiness result.
// Wire field names live on handler DTOs, not on domain types.
type HealthStatus struct {
	Status string
}
