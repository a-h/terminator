package main

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/a-h/terminator/integration"
	"github.com/blang/semver"
)

func testIsHealthy(t *testing.T) {
	instances := []integration.Instance{
		integration.Instance{
			ID:             "A",
			LifecycleState: "InService",
			HealthStatus:   "Healthy",
		},
		integration.Instance{
			ID:             "B",
			LifecycleState: "OutOfService",
			HealthStatus:   "Healthy",
		},
		integration.Instance{
			ID:             "C",
			LifecycleState: "InService",
			HealthStatus:   "Unhealthy",
		},
	}

	actual := make([]bool, len(instances))

	for idx, in := range instances {
		actual[idx] = isHealthy(in)
	}

	expected := []bool{true, false, false}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected the health of instances %+v to be %+v, but was %+v",
			instances,
			expected, actual)
	}
}

func createTestData(customVersions map[string]string, customTimes map[string]time.Time) *MockProvider {
	groups := []integration.AutoScalingGroup{
		integration.AutoScalingGroup{
			Name: "Group1",
			Instances: []integration.Instance{
				integration.Instance{
					ID:             "A",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
				integration.Instance{
					ID:             "B",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
				integration.Instance{
					ID:             "C",
					LifecycleState: "OutOfService",
					HealthStatus:   "Healthy",
				},
			},
		},
		integration.AutoScalingGroup{
			Name: "Group2",
			Instances: []integration.Instance{
				integration.Instance{
					ID:             "D",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
				integration.Instance{
					ID:             "E",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
				integration.Instance{
					ID:             "F",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
				integration.Instance{
					ID:             "G",
					LifecycleState: "InService",
					HealthStatus:   "Healthy",
				},
			},
		},
	}

	return NewMockProvider(groups, "1.1.0", customVersions, time.Now(), customTimes)
}

func TestSuite(t *testing.T) {
	tests := []struct {
		name                 string
		customVersions       map[string]string
		customTimes          map[string]time.Time
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
			expectedTerminations: []string{"A", "B", "C", "D", "E", "F", "G"},
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
			expectedTerminations: []string{"B", "C", "E", "F", "G"},
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
			// Group1 only has two healthy servers.
			// Group2 has DEFG, so it can lose F & G
			expectedTerminations: []string{"F", "G"},
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
			expectedTerminations: []string{"B", "D", "E", "G"},
		},
		{
			name: "Don't delete too many old versions and potentially take the service down!",
			customVersions: map[string]string{
				"F": "1.4.0",
			},
			p: parameters{
				region:                   "europa-westmoreland-1",
				minimumInstanceCount:     3,
				onlyTerminateOldVersions: true,
				versionURL:               "/version",
				isDryRun:                 false,
			},
			// Group1 should be left alone completely, because all versions are equal.
			// Group2 has 4 healthy, active servers, only one of which is running the latest version.
			// So, only one server should be taken out... that server should be the oldest.
			expectedTerminations: []string{"D"},
		},
	}

	for _, test := range tests {
		mp := createTestData(test.customVersions, test.customTimes)

		// Act.
		terminate(mp, test.p)

		// Assert.
		sort.Strings(test.expectedTerminations)
		sort.Strings(mp.TerminatedInstances)
		if !equal(mp.TerminatedInstances, test.expectedTerminations) {
			t.Errorf("For test \"%s\" with paramaters %+v and custom version map %v, expected %+v to be terminated, but got %+v",
				test.name, test.p, test.customVersions, test.expectedTerminations, mp.TerminatedInstances)
		}
	}
}

func equal(a []string, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil && b != nil {
		return false
	}

	if a != nil && b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func NewMockProvider(groups []integration.AutoScalingGroup,
	defaultVersionNumber string, alternativeVersionNumbers map[string]string,
	defaultLaunchTime time.Time, alternativeLaunchTimes map[string]time.Time) *MockProvider {
	mp := &MockProvider{
		DescribeAutoScalingGroupsFunc: func() ([]integration.AutoScalingGroup, error) { return groups, nil },
		GetDetailFunc: func(instanceID string, scheme string, port int, endpoint string) (*integration.InstanceDetail, error) {
			vs := defaultVersionNumber

			if av, ok := alternativeVersionNumbers[instanceID]; ok {
				vs = av
			}

			lt := defaultLaunchTime

			if alt, ok := alternativeLaunchTimes[instanceID]; ok {
				lt = alt
			}

			version, err := semver.Make(vs)

			if err != nil {
				return nil, err
			}

			return &integration.InstanceDetail{
				ID:            instanceID,
				VersionNumber: version,
				LaunchTime:    lt,
			}, nil
		},
		TerminatedInstances: []string{},
	}

	return mp
}

type MockProvider struct {
	TerminatedInstances           []string
	DescribeAutoScalingGroupsFunc func() ([]integration.AutoScalingGroup, error)
	GetDetailFunc                 func(instanceID string, scheme string, port int, endpoint string) (*integration.InstanceDetail, error)
	TerminateInstancesFunc        func(instanceIDs []string) error
}

func (p *MockProvider) DescribeAutoScalingGroups() ([]integration.AutoScalingGroup, error) {
	return p.DescribeAutoScalingGroupsFunc()
}

func (p *MockProvider) GetDetail(instanceID string, scheme string, port int, endpoint string) (*integration.InstanceDetail, error) {
	return p.GetDetailFunc(instanceID, scheme, port, endpoint)
}

func (p *MockProvider) TerminateInstances(instanceIDs []string) error {
	p.TerminatedInstances = append(p.TerminatedInstances, instanceIDs...)

	return nil
}

func TestThatInitialVersionsAreLow(t *testing.T) {
	initial := semver.Version{}
	any, _ := semver.Make("0.0.1")

	if !any.GT(initial) {
		t.Error("version 0.0.1 should be higher than the default of 0.0.0.")
	}
}

func TestTakeAtMost(t *testing.T) {
	v1, _ := semver.New("1.0.0")
	details := integration.InstanceDetails{
		{
			ID:            "A",
			VersionNumber: *v1,
			LaunchTime:    time.Now(),
		},
		{
			ID:            "B",
			VersionNumber: *v1,
			LaunchTime:    time.Now(),
		},
		{
			ID:            "C",
			VersionNumber: *v1,
			LaunchTime:    time.Now(),
		},
	}

	tests := []struct {
		take     int
		expected int
	}{
		{
			take:     1,
			expected: 1,
		},
		{
			take:     2,
			expected: 2,
		},

		{
			take:     3,
			expected: 3,
		},

		{
			take:     4,
			expected: 3,
		},
	}

	for _, test := range tests {
		result := takeAtMost(details, test.take)

		if len(result) != test.expected {
			t.Errorf("Expected to take %d, but took %d.", test.expected, len(result))
		}
	}
}

func TestGetOldestVersions(t *testing.T) {
	// (details integration.InstanceDetails, maximumOldVersions int) integration.InstanceDetails {
	oldVersion, _ := semver.New("1.0.0")
	newVersion, _ := semver.New("1.1.0")

	details := integration.InstanceDetails{
		integration.InstanceDetail{
			ID:            "A",
			VersionNumber: *oldVersion,
			LaunchTime:    time.Now(),
		},
		integration.InstanceDetail{
			ID:            "B",
			VersionNumber: *oldVersion,
			LaunchTime:    time.Now().Add(-1 * time.Hour),
		},
		integration.InstanceDetail{
			ID:            "C",
			VersionNumber: *newVersion,
			LaunchTime:    time.Now(),
		},
	}

	lowest, highest, result := getOldestVersions(details, 1)

	if !lowest.EQ(*oldVersion) {
		t.Errorf("The lowest version should be %-v but was %-v", oldVersion, lowest)
	}

	if !highest.EQ(*newVersion) {
		t.Errorf("The highest version should be %-v but was %-v", newVersion, highest)
	}

	if len(result) != 1 || result[0].ID != "B" {
		t.Errorf("Only the single oldest instance with the old version should be returned but %+v was returned.", result)
	}
}

func TestSortingInstanceDetailsByTime(t *testing.T) {
	v, _ := semver.New("1.0.0")

	instanceDetails := integration.InstanceDetails{
		integration.InstanceDetail{
			ID:            "A",
			VersionNumber: *v,
			LaunchTime:    time.Now().Add(3 * time.Hour),
		},
		integration.InstanceDetail{
			ID:            "B",
			VersionNumber: *v,
			LaunchTime:    time.Now().Add(-1 * time.Hour),
		},
		integration.InstanceDetail{
			ID:            "C",
			VersionNumber: *v,
			LaunchTime:    time.Now(),
		},
	}

	sort.Sort(instanceDetails)

	orderedIds := make([]string, len(instanceDetails))
	for idx, v := range instanceDetails {
		orderedIds[idx] = v.ID
	}

	expected := []string{"B", "C", "A"}
	if !reflect.DeepEqual(orderedIds, expected) {
		t.Errorf("Expected %+v but got %+v", expected, orderedIds)
	}
}

func TestSemverParsingFromGitDescribe(t *testing.T) {
	// Test the output from `git describe --tags --long`
	_, err := semver.New("0.0.3-0-g03d102e")

	if err != nil {
		t.Fatal(err)
	}
}

func TestSortingInstanceDetailsByTimeAndVersion(t *testing.T) {
	v1, _ := semver.New("1.0.0")
	v2, _ := semver.New("2.0.0")

	instanceDetails := integration.InstanceDetails{
		integration.InstanceDetail{
			ID:            "A",
			VersionNumber: *v2,
			LaunchTime:    time.Now().Add(-3 * time.Hour),
		},
		integration.InstanceDetail{
			ID:            "B",
			VersionNumber: *v1,
			LaunchTime:    time.Now().Add(-1 * time.Hour),
		},
		integration.InstanceDetail{
			ID:            "C",
			VersionNumber: *v1,
			LaunchTime:    time.Now(),
		},
	}

	sort.Sort(instanceDetails)

	orderedIds := make([]string, len(instanceDetails))
	for idx, v := range instanceDetails {
		orderedIds[idx] = v.ID
	}

	// Should be B, C, A because:
	//  B is v1 and an hour old
	//  C is V1 and is seconds old
	//  A is 3 hours hold, but is V2, so this takes precedence.
	expected := []string{"B", "C", "A"}
	if !reflect.DeepEqual(orderedIds, expected) {
		t.Errorf("Expected %+v but got %+v", expected, orderedIds)
	}
}
