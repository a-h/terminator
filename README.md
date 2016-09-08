AWS Terminator
==============
[![Build Status](https://travis-ci.org/a-h/terminator.svg?branch=master)](https://travis-ci.org/a-h/terminator)

A Go program which terminates healthy instances from autoscaling groups based on version rules.

To build a static binary to run on AWS from your Mac or Windows PC.

```bash
GOOS=linux GOARCH=amd64 go build main.go
```

Example Output
--------------
```
Retrieving data on autoscaling groups...
Retrieved information on groups [asg_api asg_web dev_asg_web].
asg_api => 3 healthy instances, 0 unhealthy instances
asg_api => terminating 2 of 3 instances
asg_api => complete
asg_web => 3 healthy instances, 0 unhealthy instances
asg_web => terminating 2 of 3 instances
asg_web => complete
dev_asg_web => 3 healthy instances, 0 unhealthy instances
dev_asg_web => terminating 2 of 3 instances
dev_asg_web => complete
Complete.
```
