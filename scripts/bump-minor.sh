#!/bin/bash
git fetch origin --tags

tag=$(git tag -l --sort=-version:refname  | head -n1 | awk -F. '{$2++; $3=0; print $1"."$2"."$3}')
git tag $tag
git push -f origin HEAD
echo "sleep 3 seconds for CI to detect tag pushing"
sleep 3
git push origin $tag
