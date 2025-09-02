#!/bin/bash

set -e

if [[ -n "$(git status --porcelain)" ]] ; then
    echo "You have uncommitted changes. Please commit the changes."
    git status --porcelain
    exit 1
fi
