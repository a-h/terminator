package main

import (
  "fmt"
  "sort"

  "github.com/a-h/terminator/integration"
  "github.com/blang/semver"
)

func Terminate(cloud integration.CloudProvider, p parameters) []string {
  if p.isDryRun {
    fmt.Println("[DRY RUN] Terminator activated. Searching for Sarah Connor...")
  } else {
    fmt.Println("Terminator activated. Searching for Sarah Connor...")
  }

  terminatedInstances := []string{}

  groups, err := cloud.DescribeAutoScalingGroups(
    p.autoScalingGroups,
    p.scheme,
    p.port,
    p.versionURL)

  if err != nil {
    fmt.Printf("Failed to get auto scaling groups, %+v. Exiting...\n", err)
    return nil
  }

	fmt.Println("Working on groups ", getGroupNames(groups))

  canonicalVersion, err := semver.Make(p.canonical)
  if err != nil {
    fmt.Errorf("Failed to parse canonical version, %+v\n", err)
    return nil
  }

	for _, g := range groups {
    targets, err := g.GetTargetInstances(canonicalVersion, p.minimumInstanceCount)
    if err != nil {
      fmt.Errorf("%s => Failed to flag instances for removal, %+v\n", g.Name, err)
      continue
    }

    if len(targets) <= 0 {
      fmt.Printf("%s => no action taken, no instances to terminate\n", g.Name)
      continue
    }

    fmt.Printf("%s => terminating %d of %d instances\n", g.Name, len(targets), len(g.Instances))

    fmt.Printf("%s => terminating instance ids %-v\n", g.Name, targets)

    if p.isDryRun {
      fmt.Printf("%s => no action taken, set --isDryRun=false to execute\n", g.Name)
      continue
    }

    err = cloud.TerminateInstances(targets)

    if err != nil {
      fmt.Errorf("%s => failed to terminate instances with error - %s\n", g.Name, err)
    } else {
      terminatedInstances = append(terminatedInstances, targets...)
      fmt.Printf("%s => complete\n", g.Name)
    }
	}

	fmt.Println("Completed termination of all groups ", getGroupNames(groups))
	return terminatedInstances
}

func getGroupNames(grps []integration.AutoScalingGroup) []string {
	names := make([]string, len(grps))

	for i, g := range grps {
		names[i] = g.Name
	}

	return names
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


func takeAtMost(details integration.InstanceDetails, most int) integration.InstanceDetails {
	if len(details) <= most {
		return details
	}

	return details[:most]
}
