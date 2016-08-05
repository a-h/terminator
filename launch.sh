#/bin/bash
ECHO building for Linux
GOOS=linux GOARCH=amd64 go build ./...
ECHO "done"
