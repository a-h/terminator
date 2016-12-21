package main

import (
  "github.com/blang/semver"
)

type VersionDetails struct {
  canonical semver.Version
  highest   semver.Version
  lowest    semver.Version
  ids       []string
}
//
// func QueryVersionDetails(cloud integration.CloudProvider, instances []Instance, scheme string, port int, path string) (VersionDetails, error) {
//
// }
