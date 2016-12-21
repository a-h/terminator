package main

import (
  "fmt"
  "strings"
  "sort"

  "github.com/a-h/terminator/integration"
  "github.com/blang/semver"
)

func Terminate(cloud integration.CloudProvider, p parameters) []string {
  terminatedInstances := []string{}

	allGroups, err := cloud.DescribeAutoScalingGroups()

	if err != nil {
		fmt.Println("Failed to get the description of all autoscaling groups, ", err)
		return terminatedInstances
	}

	groups := filterByName(allGroups, []string(p.autoScalingGroups))

	if len(p.autoScalingGroups) > 0 {
		fmt.Printf("Filtering groups %+v by expression %+v.\n", getGroupNames(allGroups), []string(p.autoScalingGroups))
	}

	fmt.Printf("Working on groups %-v.\n", getGroupNames(groups))

	for _, g := range groups {
		healthy, unhealthy := categoriseInstances(g.Instances, p.minimumInstanceCount)

		fmt.Printf("%s => %d healthy instances, %d unhealthy instances => healthy: %+v - unhealthy: %+v\n",
			g.Name, len(healthy), len(unhealthy),
			healthy,
			unhealthy)

		if len(healthy) <= p.minimumInstanceCount {
			fmt.Printf("%s => no action taken\n", g.Name)

			continue
		}

		var instanceIdsToTerminate []string

		if p.onlyTerminateOldVersions {
			fmt.Printf("%s => only terminating old versions\n", g.Name)

			maximumOldVersions := len(healthy) - p.minimumInstanceCount
			var lowestVersion, highestVersion semver.Version
			var err error
			lowestVersion, highestVersion, instanceIdsToTerminate, err = getOldestIDs(cloud, healthy, p.scheme, p.port, p.versionURL, maximumOldVersions)

			if err != nil {
				fmt.Printf("%s => failed to get version data with error %-v\n", g.Name, err)
			}

			fmt.Printf("%s => lowest version %-v, highest version %-v, %d instances to terminate\n", g.Name, lowestVersion, highestVersion, len(instanceIdsToTerminate))
		} else {
			fmt.Printf("%s => terminating oldest instance(s) regardless of version\n", g.Name)

			instancesToTerminate := append(healthy[p.minimumInstanceCount:], unhealthy...)
			instanceIdsToTerminate = getInstanceIDs(instancesToTerminate)
		}

		if len(instanceIdsToTerminate) > 0 {
			fmt.Printf("%s => terminating %d of %d instances\n", g.Name, len(instanceIdsToTerminate), len(g.Instances))

			fmt.Printf("%s => terminating instance ids %-v\n", g.Name, instanceIdsToTerminate)

			if p.isDryRun {
				fmt.Printf("%s => no action taken, set --isDryRun=false to execute\n", g.Name)
			} else {
				terminatedInstances = append(terminatedInstances, instanceIdsToTerminate...)
				err = cloud.TerminateInstances(instanceIdsToTerminate)

				if err != nil {
					fmt.Printf("%s => failed to terminate instances with error - %s\n", g.Name, err)
				} else {
					fmt.Printf("%s => complete\n", g.Name)
				}
			}
		}
	}

	fmt.Println("Complete.")
	return terminatedInstances
}

func getGroupNames(grps []integration.AutoScalingGroup) []string {
	names := make([]string, len(grps))

	for i, g := range grps {
		names[i] = g.Name
	}

	return names
}

func filterByName(grps []integration.AutoScalingGroup, namesToInclude []string) []integration.AutoScalingGroup {
	if len(namesToInclude) == 0 {
		return grps
	}

	var op []integration.AutoScalingGroup

	for _, g := range grps {
		for _, n := range namesToInclude {
			if strings.EqualFold(g.Name, n) {
				op = append(op, g)
			}
		}
	}

	return op
}

func categoriseInstances(instances []integration.Instance, minimumInstanceCount int) (healthyInstances []integration.Instance, otherInstances []integration.Instance) {
	healthyInstances = []integration.Instance{}
	otherInstances = []integration.Instance{}

	for _, instance := range instances {
		if isHealthy(instance) {
			healthyInstances = append(healthyInstances, instance)
		} else {
			otherInstances = append(otherInstances, instance)
		}
	}

	return healthyInstances, otherInstances
}

func isHealthy(instance integration.Instance) bool {
	return strings.EqualFold(instance.HealthStatus, "Healthy") &&
		strings.EqualFold(instance.LifecycleState, "InService")
}

func getInstanceIDs(instances []integration.Instance) []string {
	ids := make([]string, len(instances))

	for idx, instance := range instances {
		ids[idx] = instance.ID
	}

	return ids
}

func getOldestIDs(cloud integration.CloudProvider, instances []integration.Instance, scheme string, port int, path string, maximumInstances int) (lowestVersion semver.Version, highestVersion semver.Version, ids []string, err error) {
	details, err := getDetails(cloud, instances, scheme, port, path)

	if err != nil {
		return semver.Version{}, semver.Version{}, nil, err
	}

	lowestVersion, highestVersion, details = getOldestVersions(details, maximumInstances)

	ids = make([]string, len(details))

	for i, v := range details {
		ids[i] = v.ID
	}

	return lowestVersion, highestVersion, ids, err
}

func getDetails(cloud integration.CloudProvider, instances []integration.Instance, scheme string, port int, path string) (integration.InstanceDetails, error) {
	details := integration.InstanceDetails{}

	for _, instance := range instances {
		detail, err := cloud.GetDetail(instance.ID, scheme, port, path)

		if err != nil {
			return nil, err
		}

		details = append(details, *detail)
	}

	sort.Sort(details)

	return details, nil
}

func getOldestVersions(details integration.InstanceDetails, maximumOldVersions int) (lowestVersion semver.Version, highestVersion semver.Version, filtered integration.InstanceDetails) {
	for _, instance := range details {
		if instance.VersionNumber.GT(highestVersion) {
			highestVersion = instance.VersionNumber
		}
	}

	// Collect instances which don't have the latest version.
	lowestVersion = highestVersion
	oldInstances := integration.InstanceDetails{}

	for _, instance := range details {
		if instance.VersionNumber.LT(highestVersion) {
			oldInstances = append(oldInstances, instance)
		}

		if instance.VersionNumber.LT(lowestVersion) {
			lowestVersion = instance.VersionNumber
		}
	}

	// Sort them by version and age to delete the old ones first.
	sort.Sort(oldInstances)

	return lowestVersion, highestVersion, takeAtMost(oldInstances, maximumOldVersions)
}

func takeAtMost(details integration.InstanceDetails, most int) integration.InstanceDetails {
	if len(details) <= most {
		return details
	}

	return details[:most]
}

func getCanonicalVersion(cloud integration.CloudProvider) (VersionDetails, error) {
  data, err := cloud.GetObject("westfield-build", "dev/version.txt")

  if err != nil {
    return VersionDetails{}, err
  }

  ver, err := semver.Make(data)

  if err != nil {
    return VersionDetails{}, err
  }

  versionDetails := VersionDetails{
    canonical: ver,
  }

  return versionDetails, nil
}
