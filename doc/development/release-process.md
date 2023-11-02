# Release Process

Create a new branch for the new release:

```shell
git checkout -b release/v1.39.0 origin/main
```

## 1. Run the release test script

```shell
bash scripts/release-test.sh
```

## 2. Prepare the release note

You need to prepare the release note for your next release version.

The release notes are placed in the `doc/release` directory.

If your next version is `v1.20.2`, then you should put the release note in the following file:

```
doc/release/v1.20.2.md
```

Run changelog script to generate a changelog template:

```sh
bash utils/changelog.sh > doc/release/v1.20.2.md
```

Edit your changelog.

## 3. Make the release

Run the following command to create the release:

```sh
make version VERSION=v1.20.2
```

The above command will:

- Update and compile the migration scripts into go files.
- Bump the version name in the go code.
- Run git tag to create the tag.
- Run git push to push the created tag.

You can go to <https://github.com/c9s/bbgo/releases/v1.20.2> to modify the changelog
