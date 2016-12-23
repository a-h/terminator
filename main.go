package main

import (
	"flag"
	"fmt"

	"github.com/a-h/terminator/integration"
)

var version string

var regionFlag = flag.String("region", "eu-west-1", "Specifies the default region used.")
var isDryRunFlag = flag.Bool("isDryRun", true, "Specifies whether to do a dry run (test) of the termination. If this is specified, the termination will not occur.")
var minimumInstanceCountFlag = flag.Int("minimumInstanceCount", 1, "Specifies the minimum number of instances to leave in the auto-scaling group.")
var schemeFlag = flag.String("scheme", "http", "Chooses the scheme, e.g. http or https.")
var portFlag = flag.Int("port", 80, "The TCP port to run communications over.")
var versionURLFlag = flag.String("path", "/version/", "Specifies the URL path which will be connected to (after the private IP address of the instance. The expectation is a version number should be returned, e.g. 1.1.4")
var versionFlag = flag.Bool("version", false, "When set, just displays the version and quits.")

var canonicalFlag = flag.String("canonical", "1.0.0", "The canonical version to check against when terminating instances.")

var autoScalingGroupsFlag AsgParams

func init() {
	// Tie the command-line flag to the intervalFlag variable and
	// set a usage message.
	flag.Var(&autoScalingGroupsFlag, "autoScalingGroups", "Comma-separated list of autoscaling group names.")
}

type parameters struct {
	region                   string
	isDryRun                 bool
	minimumInstanceCount     int
	scheme                   string
	port                     int
	versionURL               string
	autoScalingGroups        AsgParams
	canonical								 string
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
		scheme:            				*schemeFlag,
		port:              				*portFlag,
		versionURL:        				*versionURLFlag,
		autoScalingGroups: 				autoScalingGroupsFlag,
		canonical:								*canonicalFlag,
	}

	Terminate(aws, p)
}
