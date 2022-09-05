#!/bin/bash
PACKAGE_NAME=version
REF=$(git show -s --format=%h -1)

if [[ -z $VERSION ]] ; then
    VERSION=$(git describe --tags)
fi

VERSION=$VERSION-$REF

if [[ -z $BUILD_FLAGS ]] ; then
   BUILD_FLAGS=release
fi


if [[ -n $VERSION_SUFFIX ]] ; then
    VERSION=${VERSION}${VERSION_SUFFIX}
fi

cat <<END
//go:build $BUILD_FLAGS
// +build $BUILD_FLAGS

package $PACKAGE_NAME

const Version = "${VERSION}"

const VersionGitRef = "${REF}"
END
