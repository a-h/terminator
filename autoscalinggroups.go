package main

import (
	"errors"
	"strings"
)

type autoScalingGroups []string

func (a *autoScalingGroups) String() string {
	return strings.Join(*a, ",")
}

func (a *autoScalingGroups) Set(value string) error {
	if len(*a) > 0 {
		return errors.New("autoScalingGroups flag already set")
	}
	for _, v := range strings.Split(value, ",") {
		*a = append(*a, v)
	}

	return nil
}
