#!/bin/bash
PACKAGE_NAME=version
REF=$(git show -s --format=%h -1)
VERSION=$VERSION-$REF

if [[ -z $VERSION ]] ; then
    VERSION=$(git describe --tags)
fi

if [[ -n $VERSION_SUFFIX ]] ; then
    VERSION=${VERSION}${VERSION_SUFFIX}
fi

cat <<END
// +build release

package $PACKAGE_NAME

const Version = "${VERSION}"

const VersionGitRef = "${REF}"

END
