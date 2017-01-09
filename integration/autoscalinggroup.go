package integration

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/blang/semver"
)

// AutoScalingGroup represents an autoscaling group.
type AutoScalingGroup struct {
	Name      			string
	Instances 			[]Instance
	InstanceDetails InstanceDetails
}

func NewAutoScalingGroup(name string, instances []*autoscaling.Instance, instanceDetails InstanceDetails) AutoScalingGroup {
	asg := AutoScalingGroup {
		Name: name,
		Instances: make([]Instance, len(instances)),
		InstanceDetails: instanceDetails,
	}

	for i, awsInstance := range instances {
		asg.Instances[i] = Instance {
			ID:							aws.StringValue(awsInstance.InstanceId),
			HealthStatus: 	aws.StringValue(awsInstance.HealthStatus),
			LifecycleState: aws.StringValue(awsInstance.LifecycleState),
		}
	}

	return asg
}

func (group AutoScalingGroup) GetTargetInstances(canonical semver.Version, minimumInstanceCount int) ([]string, error) {
	start := time.Now()
	healthy, unhealthy := categoriseInstances(group.Instances, minimumInstanceCount)

  fmt.Printf("%s => %d healthy instances, %d unhealthy instances\n\thealthy: %+v\n\tunhealthy: %+v\n",
    group.Name, len(healthy), len(unhealthy),
    healthy,
    unhealthy)

	if len(healthy) <= minimumInstanceCount {
    fmt.Printf("%s => not enough healthy instances\n", group.Name)
    return []string{}, nil
  }

	if len(unhealthy) > 0 || len(healthy) != len(group.InstanceDetails) {
		fmt.Printf("%s => couldn't get all instance details, some instances may still be starting\n", group.Name)
		return []string{}, nil
	}

	fmt.Printf("%s => finding instances that don't match version %s\n", group.Name, canonical)
	var mismatchedInstances []string

	for i, details := range group.InstanceDetails {
		if details.VersionNumber.LT(canonical) || details.VersionNumber.GT(canonical) {
			mismatchedInstances = append(mismatchedInstances, group.Instances[i].ID)
		}
	}

	if len(mismatchedInstances) == 0 {
		fmt.Println("time: AutoScalingGroup.GetTargetInstances() ", time.Since(start))
		fmt.Printf("%s => no mismatched instances detected\n", group.Name)
		return []string{}, nil
	}

	maximum := len(group.Instances) - minimumInstanceCount

	// Priority order to keep (NOT terminate) instances:
	// - Healthy, Mismatched, Unhealthy
	instanceIdsToTerminate := removeDuplicates(append(mismatchedInstances, getInstanceIDs(healthy[minimumInstanceCount:])...))

	fmt.Println("time: AutoScalingGroup.GetTargetInstances() ", time.Since(start))

	if len(instanceIdsToTerminate) <= maximum {
		return instanceIdsToTerminate, nil
	}

	return instanceIdsToTerminate[:maximum], nil
}

func categoriseInstances(instances []Instance, minimumInstanceCount int) (healthyInstances []Instance, otherInstances []Instance) {
	healthyInstances = []Instance{}
	otherInstances = []Instance{}

	for _, instance := range instances {
		if instance.IsHealthy() {
			healthyInstances = append(healthyInstances, instance)
		} else {
			otherInstances = append(otherInstances, instance)
		}
	}

	return healthyInstances, otherInstances
}

func getInstanceIDs(instances []Instance) []string {
	ids := make([]string, len(instances))

	for idx, instance := range instances {
		ids[idx] = instance.ID
	}

	return ids
}

func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == false {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}

	return result
}
