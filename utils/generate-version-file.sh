#!/bin/bash

packageName=version
version=$(git describe --tags)

cat <<END
// +build release

package $packageName

const Version = "${version}"

END
