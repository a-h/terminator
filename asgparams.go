package main

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
)

type AsgParams []string

func (a *AsgParams) String() string {
	return strings.Join(*a, ",")
}

func (a *AsgParams) Set(value string) error {
	if len(*a) > 0 {
		return errors.New("autoScalingGroups flag already set")
	}
	for _, v := range strings.Split(value, ",") {
		*a = append(*a, v)
	}

	return nil
}

func (a *AsgParams) ToAwsStrings() []*string {
	var b = []string(*a)
	var result = make([]*string, len(b))

	for i, str := range b {
		result[i] = aws.String(str)
	}

	return result
}
