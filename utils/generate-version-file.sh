#!/bin/bash

PACKAGE_NAME=version

if [[ -z $VERSION ]] ; then
    VERSION=$(git describe --tags)
fi

cat <<END
// +build release

package $PACKAGE_NAME

const Version = "${VERSION}"

END
