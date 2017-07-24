#!/bin/bash
test -n "$TRAVIS_TAG" && curl -sL https://git.io/goreleaser | bash
