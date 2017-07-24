#!/bin/bash
echo "Deploying new version"
test -n "$TRAVIS_TAG" && curl -sL https://git.io/goreleaser | bash
