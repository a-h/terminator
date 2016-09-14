package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/a-h/terminator/integration"
	"github.com/blang/semver"
)

var version string

var regionFlag = flag.String("region", "eu-west-1", "Specifies the default region used.")
var isDryRunFlag = flag.Bool("isDryRun", true, "Specifies whether to do a dry run (test) of the termination. If this is specified, the termination will not occur.")
var minimumInstanceCountFlag = flag.Int("minimumInstanceCount", 1, "Specifies the minimum number of instances to leave in the auto-scaling group.")
var onlyTerminateOldVersionsFlag = flag.Bool("terminateOldVersions", true, "When set to true, the program checks the version URL. If all versions match, no action is taken. If the versions don't match, instances with the oldest version numbers are terminated.")
var schemeFlag = flag.String("scheme", "http", "Chooses the scheme, e.g. http or https.")
var portFlag = flag.Int("port", 80, "The TCP port to run communications over.")
var versionURLFlag = flag.String("path", "/version/", "Specifies the URL path which will be connected to (after the private IP address of the instance. The expectation is a version number should be returned, e.g. 1.1.4")
var versionFlag = flag.Bool("version", false, "When set, just displays the version and quits.")

var autoScalingGroupsFlag autoScalingGroups

func init() {
	// Tie the command-line flag to the intervalFlag variable and
	// set a usage message.
	flag.Var(&autoScalingGroupsFlag, "autoScalingGroups", "Comma-separated list of autoscaling group names.")
}

type parameters struct {
	region                   string
	isDryRun                 bool
	minimumInstanceCount     int
	onlyTerminateOldVersions bool
	scheme                   string
	port                     int
	versionURL               string
	autoScalingGroups        autoScalingGroups
}

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		return
	}

	aws, err := integration.NewAWSProvider(*regionFlag)

	if err != nil {
		fmt.Println("Failed to create an AWS session, ", err)
		return
	}

	p := parameters{
		region:                   *regionFlag,
		isDryRun:                 *isDryRunFlag,
		minimumInstanceCount:     *minimumInstanceCountFlag,
		onlyTerminateOldVersions: *onlyTerminateOldVersionsFlag,
		scheme:            *schemeFlag,
		port:              *portFlag,
		versionURL:        *versionURLFlag,
		autoScalingGroups: autoScalingGroupsFlag,
	}

	terminate(aws, p)
}

// Terminates instances, returns the list of terminated instances.
func terminate(cloud integration.CloudProvider, p parameters) []string {
	terminatedInstances := []string{}

	allGroups, err := cloud.DescribeAutoScalingGroups()

	if err != nil {
		fmt.Println("Failed to get the description of all autoscaling groups, ", err)
		return terminatedInstances
	}

	groups := filterByName(allGroups, p.autoScalingGroups)

	if len(p.autoScalingGroups) > 0 {
		fmt.Printf("Filtering groups %-v by expression %-v.\n", getGroupNames(allGroups), p.autoScalingGroups)
	}

	fmt.Printf("Working on groups %-v.\n", getGroupNames(groups))

	for _, g := range groups {
		healthy, unhealthy := categoriseInstances(g.Instances, p.minimumInstanceCount)

		fmt.Printf("%s => %d healthy instances, %d unhealthy instances => healthy: %+v - unhealthy: %+v\n",
			g.Name, len(healthy), len(unhealthy),
			healthy,
			unhealthy)

		if len(healthy) > p.minimumInstanceCount {
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

				fmt.Printf("%s => lowest version %-v, highest version %-v\n", g.Name, lowestVersion, highestVersion)
			} else {
				fmt.Printf("%s => terminating oldest instances\n", g.Name)

				instancesToTerminate := append(healthy[p.minimumInstanceCount:], unhealthy...)
				instanceIdsToTerminate = getInstanceIDs(instancesToTerminate)
			}

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
		} else {
			fmt.Printf("%s => no action taken\n", g.Name)
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

	op := []integration.AutoScalingGroup{}

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
