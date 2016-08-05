#/bin/bash
ECHO building
GOOS=linux GOARCH=amd64 go build ./...
ECHO copying
cp terminator ~/Documents/mywestfield/Infrastructure/
ECHO "done"