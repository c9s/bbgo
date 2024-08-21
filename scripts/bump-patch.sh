#!/bin/bash
set -e
git fetch origin
git fetch origin --tags

tag=$(git tag -l --sort=-version:refname  | head -n1 | awk -F . '{OFS="."; $NF+=1; print $0}')
git rebase origin/main
git tag $tag
git push -f origin HEAD
echo "sleep 3 seconds for CI to detect tag pushing"
sleep 3
git push origin $tag
