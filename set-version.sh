#!/bin/sh
latestTag=$(git describe --tags)
echo "Updating version file with new tag: $latestTag"
echo "package version" > src/version/version.go
echo "" >> src/version/version.go
echo "const Version = \"$latestTag\"" >> src/version/version.go
