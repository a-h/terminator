package integration

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// CloudProvider provides all of the methods required to integrate with AWS.
type CloudProvider interface {
	// DescribeAutoScalingGroups provides information about the available auto-scaling groups.
	DescribeAutoScalingGroups() ([]AutoScalingGroup, error)
	// GetVersionNumber returns the version number returned by hitting the provided endpoint in the form:
	// {scheme}://{ec2.private_ip}:{port}{endpoint}
	// instanceID refers to the ID of the AWS EC2 instance
	// scheme is the protocol - http or https
	// port is the TCP port, e.g. 80 or 443
	// URL is the URL e.g. /version
	GetVersionNumber(instanceID string, scheme string, port int, endpoint string) (string, error)
	// TerminateInstances terminates the given instances.
	TerminateInstances(instanceIDs []string) error
}

// AutoScalingGroup represents an autoscaling group.
type AutoScalingGroup struct {
	Name      string
	Instances []Instance
}

// Instance represents an EC2 instance.
type Instance struct {
	ID             string
	HealthStatus   string
	LifecycleState string
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
func (p *AWSProvider) DescribeAutoScalingGroups() ([]AutoScalingGroup, error) {
	fmt.Println("Retrieving data on autoscaling groups...")
	svc := autoscaling.New(p.session)

	groups, err := svc.DescribeAutoScalingGroups(nil)

	if err != nil {
		return nil, fmt.Errorf("failed to get the description of all autoscaling groups, %-v", err)
	}

	rv := make([]AutoScalingGroup, len(groups.AutoScalingGroups))

	for i, g := range groups.AutoScalingGroups {
		// Create the group.
		asg := &AutoScalingGroup{
			Name:      aws.StringValue(g.AutoScalingGroupName),
			Instances: make([]Instance, len(g.Instances)),
		}

		// Extract the instances and add them to the mapping.
		for j, awsInstance := range g.Instances {
			asg.Instances[j] = Instance{
				ID:             aws.StringValue(awsInstance.InstanceId),
				HealthStatus:   aws.StringValue(awsInstance.HealthStatus),
				LifecycleState: aws.StringValue(awsInstance.LifecycleState),
			}
		}

		rv[i] = *asg
	}

	return rv, err
}

// GetVersionNumber returns the version number returned by hitting the provided endpoint in the form:
func (p *AWSProvider) GetVersionNumber(instanceID string, scheme string, port int, endpoint string) (string, error) {
	//TODO: Use EC2 to get the version number.
	svc := ec2.New(p.session)
	instances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: convert([]string{instanceID}),
	})

	if err != nil {
		return "", err
	}

	for _, reservation := range instances.Reservations {
		for _, instance := range reservation.Instances {
			ip := aws.StringValue(instance.PrivateIpAddress)

			complete := fmt.Sprintf("%s://%s:%d%s", scheme, ip, port, endpoint)
			u, err := url.Parse(complete)

			if err != nil {
				return "", fmt.Errorf("Failed to parse URL %s - %-v", complete, err)
			}

			versionNumber, err := getURL(u.String())

			if err != nil {
				return "", fmt.Errorf("Failed to get version number from URL %s with error %-v", complete, err)
			}

			return versionNumber, nil
		}
	}

	return "", nil
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

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	return string(body), nil
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

func convert(s []string) []*string {
	rv := make([]*string, len(s))

	for i, v := range s {
		rv[i] = &v
	}

	return rv
}
