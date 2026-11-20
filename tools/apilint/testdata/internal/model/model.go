package model

// HealthStatus is a domain type. Wire tags belong on handler DTOs.
type HealthStatus struct {
	Status string
	Code   string `json:"code"` // want `model-no-wire-tags`
}
