package integration

import (
  "time"
  "strings"

  "github.com/blang/semver"
)

// Instance represents an EC2 instance.
type Instance struct {
	ID             string
	HealthStatus   string
	LifecycleState string
}

func (instance Instance) IsHealthy() bool {
	return strings.EqualFold(instance.HealthStatus, "Healthy") &&
		strings.EqualFold(instance.LifecycleState, "InService")
}

// InstanceDetail provides information about the instance from EC2.
type InstanceDetail struct {
	ID            string
	VersionNumber semver.Version
	LaunchTime    time.Time
}

// InstanceDetails implements a sorted type for InstanceDetail.
type InstanceDetails []InstanceDetail

func (slice InstanceDetails) Len() int {
	return len(slice)
}

func (slice InstanceDetails) Less(i, j int) bool {
	return slice[i].VersionNumber.LT(slice[j].VersionNumber) || slice[i].LaunchTime.Before(slice[j].LaunchTime)
}

func (slice InstanceDetails) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}
