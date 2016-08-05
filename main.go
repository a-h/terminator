package main

import (
	"flag"
	"fmt"

	"github.com/a-h/terminator/integration"
	"github.com/blang/semver"
)

var regionFlag = flag.String("region", "eu-west-1", "Specifies the default region used.")
var isDryRunFlag = flag.Bool("isDryRun", true, "Specifies whether to do a dry run (test) of the termination. If this is specified, the termination will not occur.")
var minimumInstanceCountFlag = flag.Int("minimumInstanceCount", 1, "Specifies the minimum number of instances to leave in the auto-scaling group.")
var onlyTerminateOldVersionsFlag = flag.Bool("terminateOldVersions", true, "When set to true, the program checks the version URL. If all versions match, no action is taken. If the versions don't match, instances with the oldest version numbers are terminated.")
var schemeFlag = flag.String("scheme", "http", "Chooses the scheme, e.g. http or https.")
var portFlag = flag.Int("port", 80, "The TCP port to run communications over.")
var versionURLFlag = flag.String("path", "/Version/", "Specifies the URL path which will be connected to (after the private IP address of the instance. The expectation is a version number should be returned, e.g. 1.1.4")

type parameters struct {
	region                   string
	isDryRun                 bool
	minimumInstanceCount     int
	onlyTerminateOldVersions bool
	scheme                   string
	port                     int
	versionURL               string
}

func main() {
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
		scheme:     *schemeFlag,
		port:       *portFlag,
		versionURL: *versionURLFlag,
	}

	terminate(aws, p)
}

// Terminates instances, returns the list of terminated instances.
func terminate(cloud integration.CloudProvider, p parameters) []string {
	terminatedInstances := []string{}

	groups, err := cloud.DescribeAutoScalingGroups()

	if err != nil {
		fmt.Println("failed to get the description of all autoscaling groups,", err)
		return terminatedInstances
	}

	fmt.Printf("Retrieved information on groups %-v.\n", getGroupNames(groups))
	for _, g := range groups {
		healthy, unhealthy := categoriseInstances(g.Instances, p.minimumInstanceCount)

		fmt.Printf("%s => %d healthy instances, %d unhealthy instances\n",
			g.Name, len(healthy), len(unhealthy))

		if len(healthy) > p.minimumInstanceCount {
			var instancesToTerminate []integration.Instance

			if p.onlyTerminateOldVersions {
				maximumOldVersions := len(healthy) - p.minimumInstanceCount
				instancesToTerminate, err = getOldestVersions(cloud, healthy, maximumOldVersions, p.scheme, p.port, p.versionURL)

				if err != nil {
					fmt.Printf("%s => failed to get version data with error %-v", g.Name, err)
				}
			} else {
				instancesToTerminate = append(healthy[p.minimumInstanceCount:], unhealthy...)
			}

			fmt.Printf("%s => terminating %d of %d instances\n", g.Name, len(instancesToTerminate), len(g.Instances))

			ids := getInstanceIDs(instancesToTerminate)

			fmt.Printf("%s => terminating instance ids %-v\n", g.Name, ids)

			if p.isDryRun {
				fmt.Printf("%s => no action taken, set --isDryRun=true to execute", g.Name)
			} else {
				terminatedInstances = append(terminatedInstances, ids...)
				err = cloud.TerminateInstances(ids)

				if err != nil {
					fmt.Printf("%s => failed to terminate instances with error - %s\n", g.Name, err)
				} else {
					fmt.Printf("%s => complete\n", g.Name)
				}
			}
		} else {
			fmt.Printf("%s => no action taken.\n", g.Name)
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

func categoriseInstances(instances []integration.Instance, minimumInstanceCount int) (healthyInstances []integration.Instance, otherInstances []integration.Instance) {
	healthyInstances = []integration.Instance{}
	otherInstances = []integration.Instance{}

	for _, instance := range instances {
		hs := instance.HealthStatus
		ls := instance.LifecycleState
		if hs == "HEALTHY" && ls == "InService" {
			healthyInstances = append(healthyInstances, instance)
		} else {
			otherInstances = append(otherInstances, instance)
		}
	}

	return healthyInstances, otherInstances
}

func getInstanceIDs(instances []integration.Instance) []string {
	ids := make([]string, len(instances))

	for idx, instance := range instances {
		ids[idx] = instance.ID
	}

	return ids
}

func getOldestVersions(cloud integration.CloudProvider, instances []integration.Instance, maximumOldVersions int, scheme string, port int, path string) ([]integration.Instance, error) {
	instanceToVersionMap := make(map[integration.Instance]semver.Version)

	var highestVersion semver.Version
	for _, instance := range instances {
		vn, err := cloud.GetVersionNumber(instance.ID, scheme, port, path)

		if err != nil {
			return nil, err
		}

		v, err := semver.Make(vn)

		if v.GT(highestVersion) {
			highestVersion = v
		}

		instanceToVersionMap[instance] = v
	}

	// Collect N old versions.
	oldInstances := []integration.Instance{}

	for k, v := range instanceToVersionMap {
		if v.LT(highestVersion) {
			oldInstances = append(oldInstances, k)
		}
	}

	return oldInstances, nil
}
