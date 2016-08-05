package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/a-h/terminator/integration"
)

func createTestData(customVersions map[string]string) *MockProvider {
	groups := []integration.AutoScalingGroup{
		integration.AutoScalingGroup{
			Name: "Group1",
			Instances: []integration.Instance{
				integration.Instance{
					ID:             "A",
					LifecycleState: "InService",
					HealthStatus:   "HEALTHY",
				},
				integration.Instance{
					ID:             "B",
					LifecycleState: "InService",
					HealthStatus:   "HEALTHY",
				},
				integration.Instance{
					ID:             "C",
					LifecycleState: "OutOfService",
					HealthStatus:   "HEALTHY",
				},
			},
		},
		integration.AutoScalingGroup{
			Name: "Group2",
			Instances: []integration.Instance{
				integration.Instance{
					ID:             "D",
					LifecycleState: "InService",
					HealthStatus:   "HEALTHY",
				},
				integration.Instance{
					ID:             "E",
					LifecycleState: "InService",
					HealthStatus:   "HEALTHY",
				},
				integration.Instance{
					ID:             "F",
					LifecycleState: "InService",
					HealthStatus:   "HEALTHY",
				},
			},
		},
	}

	return NewMockProvider(groups, "1.1.0", customVersions)
}

func TestSuite(t *testing.T) {
	tests := []struct {
		name                 string
		customVersions       map[string]string
		p                    parameters
		expectedTerminations []string
	}{
		{
			name:           "Delete fully, regardless of health because min instance count is zero.",
			customVersions: map[string]string{},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     0,
				onlyTerminateOldVersions: false,
				versionURL:               "",
				isDryRun:                 false,
			},
			expectedTerminations: []string{"A", "B", "C", "D", "E", "F"},
		},
		{
			name:           "Don't delete if isDryRun is set to true.",
			customVersions: map[string]string{},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     0,
				onlyTerminateOldVersions: false,
				versionURL:               "",
				isDryRun:                 true,
			},
			expectedTerminations: []string{},
		},
		{
			name:           "Only delete each group down to the minimum.",
			customVersions: map[string]string{},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     1,
				onlyTerminateOldVersions: false,
				versionURL:               "",
				isDryRun:                 false,
			},
			expectedTerminations: []string{"B", "C", "E", "F"},
		},
		{
			name:           "Don't do anything to the group if you would leave the cluster unhealthy.",
			customVersions: map[string]string{},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     2,
				onlyTerminateOldVersions: false,
				versionURL:               "",
				isDryRun:                 false,
			},
			expectedTerminations: []string{"F"},
		},
		{
			name: "Delete the old versions!",
			customVersions: map[string]string{
				"A": "1.0.0",
			},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     1,
				onlyTerminateOldVersions: true,
				versionURL:               "/version",
				isDryRun:                 false,
			},
			expectedTerminations: []string{"A"},
		},
		{
			name: "Delete the old versions!",
			customVersions: map[string]string{
				"A": "1.3.0",
				"B": "0.9.0",
				"F": "1.4.0",
			},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     1,
				onlyTerminateOldVersions: true,
				versionURL:               "/version",
				isDryRun:                 false,
			},
			// C isn't terminated because it's OutOfService.
			expectedTerminations: []string{"B", "D", "E"},
		},
	}

	for _, test := range tests {
		mp := createTestData(test.customVersions)

		// Act.
		terminate(mp, test.p)

		// Assert.
		sortedActualTerminations := sort.StringSlice(mp.TerminatedInstances)
		sortedExpectedTerminations := sort.StringSlice(test.expectedTerminations)
		if !reflect.DeepEqual(sortedActualTerminations, sortedExpectedTerminations) {
			t.Errorf("For test \"%s\" with paramaters %-v and custom version map %-v, expected %-v to be terminated, but got %-v",
				test.name, test.p, test.customVersions, test.expectedTerminations, mp.TerminatedInstances)
		}
	}
}

func NewMockProvider(groups []integration.AutoScalingGroup, defaultVersionNumber string, alternativeVersionNumbers map[string]string) *MockProvider {
	mp := &MockProvider{
		DescribeAutoScalingGroupsFunc: func() ([]integration.AutoScalingGroup, error) { return groups, nil },
		GetVersionNumberFunc: func(instanceID string, scheme string, port int, endpoint string) (string, error) {
			if v, ok := alternativeVersionNumbers[instanceID]; ok {
				return v, nil
			}

			return defaultVersionNumber, nil
		},
		TerminatedInstances: []string{},
	}

	return mp
}

type MockProvider struct {
	TerminatedInstances           []string
	DescribeAutoScalingGroupsFunc func() ([]integration.AutoScalingGroup, error)
	GetVersionNumberFunc          func(instanceID string, scheme string, port int, endpoint string) (string, error)
	TerminateInstancesFunc        func(instanceIDs []string) error
}

func (p *MockProvider) DescribeAutoScalingGroups() ([]integration.AutoScalingGroup, error) {
	return p.DescribeAutoScalingGroupsFunc()
}

func (p *MockProvider) GetVersionNumber(instanceID string, scheme string, port int, endpoint string) (string, error) {
	return p.GetVersionNumberFunc(instanceID, scheme, port, endpoint)
}

func (p *MockProvider) TerminateInstances(instanceIDs []string) error {
	p.TerminatedInstances = append(p.TerminatedInstances, instanceIDs...)

	return nil
}
