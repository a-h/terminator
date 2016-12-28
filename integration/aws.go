package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/blang/semver"
)

// CloudProvider provides all of the methods required to integrate with AWS.
type CloudProvider interface {
	// DescribeAutoScalingGroups provides information about the available auto-scaling groups.
	DescribeAutoScalingGroups(names []string, scheme string, port int, path string) ([]AutoScalingGroup, error)
	// GetDetail returns the launch time and version number returned by accessing the EC2 API and
	// hitting the provided endpoint in the form {scheme}://{ec2.private_ip}:{port}{endpoint}
	// instanceID refers to the ID of the AWS EC2 instance
	// scheme is the protocol - http or https
	// port is the TCP port, e.g. 80 or 443
	// URL is the URL e.g. /version
	GetDetail(instanceID string, scheme string, port int, endpoint string) (*InstanceDetail, error)
	// TerminateInstances terminates the given instances.
	TerminateInstances(instanceIDs []string) error
}

// AWSProvider provides data from AWS.
type AWSProvider struct {
	session *session.Session
}

// NewAWSProvider creates an AWSProvider.
// region, the default AWS region e.g. "eu-west-1"
func NewAWSProvider(region string) (*AWSProvider, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})

	if err != nil {
		return nil, fmt.Errorf("failed to create a session, %-v", err)
	}

	return &AWSProvider{session: sess}, nil
}

// DescribeAutoScalingGroups provides information about the available auto-scaling groups.
func (p *AWSProvider) DescribeAutoScalingGroups(names []string, scheme string, port int, path string) ([]AutoScalingGroup, error) {
	fmt.Println("Retrieving data on autoscaling groups:", names)
	svc := autoscaling.New(p.session)

	awsGroups, err := svc.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: convert(names),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get the description of all autoscaling groups, %-v", err)
	}

	groups := make([]AutoScalingGroup, len(awsGroups.AutoScalingGroups))

	for i, g := range awsGroups.AutoScalingGroups {
		fmt.Printf("%s => Getting instance details for this autoscaling group.\n", aws.StringValue(g.AutoScalingGroupName))

		instanceDetails, err := p.GetInstanceDetails(g.Instances, scheme, port, path)
		if err != nil {
			fmt.Errorf("%s => Failed to get instance details, skipping this group\n", aws.StringValue(g.AutoScalingGroupName))
			continue
		}

		asg := NewAutoScalingGroup(
			aws.StringValue(g.AutoScalingGroupName),
			g.Instances,
			instanceDetails)

		fmt.Println("%s => Retrieved all instance details.", asg.Name)
		groups[i] = asg
	}

	return groups, nil
}

func (p *AWSProvider) GetInstanceDetails(instances []*autoscaling.Instance, scheme string, port int, path string) (InstanceDetails, error) {
	details := InstanceDetails{}

	for _, instance := range instances {
		detail, err := p.GetDetail(aws.StringValue(instance.InstanceId), scheme, port, path)

		if err != nil {
			return nil, err
		}

		details = append(details, *detail)
	}

	return details, nil
}

// GetDetail returns information about the instance.
func (p *AWSProvider) GetDetail(instanceID string, scheme string, port int, endpoint string) (*InstanceDetail, error) {
	svc := ec2.New(p.session)
	instances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: convert([]string{instanceID}),
	})

	if err != nil {
		return nil, err
	}

	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			ip := aws.StringValue(instance.PrivateIpAddress)

			complete := fmt.Sprintf("%s://%s:%d%s", scheme, ip, port, endpoint)
			u, err := url.Parse(complete)

			if err != nil {
				return nil, fmt.Errorf("Failed to parse URL %s - %-v", complete, err)
			}

			versionNumber, err := getURL(u.String())

			if err != nil {
				return nil, fmt.Errorf("Failed to get version number from URL %s with error %-v", complete, err)
			}

			// Trim quotes.
			versionNumber = strings.Trim(versionNumber, "\"")

			// Trim v from any version number returned from a URL.
			if strings.HasPrefix(versionNumber, "v") {
				versionNumber = versionNumber[1:]
			}

			version, err := semver.Make(versionNumber)

			if err != nil {
				return nil, fmt.Errorf("Failed to understand the version number %s with error %-v", versionNumber, err)
			}

			return &InstanceDetail{
				ID:            instanceID,
				VersionNumber: version,
				LaunchTime:    aws.TimeValue(instance.LaunchTime),
			}, nil
		}
	}

	return nil, fmt.Errorf("Could not find an instance with id %s", instanceID)
}

// TerminateInstances terminates the given instances.
func (p *AWSProvider) TerminateInstances(instanceIDs []string) error {
	params := &ec2.TerminateInstancesInput{
		InstanceIds: convert(instanceIDs),
	}

	svc := ec2.New(p.session)
	_, err := svc.TerminateInstances(params)

	return err
}

func getURL(url string) (string, error) {
	request, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(request)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func convert(s []string) []*string {
	rv := make([]*string, len(s))

	for i, v := range s {
		rv[i] = aws.String(v)
	}

	return rv
}
