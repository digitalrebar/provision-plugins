#!/usr/bin/env bash

set -e

. tools/version.sh
version="$Prepart$MajorV.$MinorV.$PatchV$Extra-$GITHASH"

DOIT=0
if [[ $version =~ ^v || $version =~ ^tip ]] ; then
    DOIT=1
fi
if [[ $version =~ travis ]] ; then
    DOIT=0
fi
if [[ $DOIT == 0 ]] ;then
    echo "Not a publishing branch."
    exit 0
fi

TOKEN=R0cketSk8ts

# Put docs in place
mkdir -p rebar-catalog/docs
cp cmds/*/*.rst rebar-catalog/docs

ls cmds | while read cmd ; do
    CONTENT=$cmd

    echo "{" > $CONTENT.json
    echo "  \"id\": \"$CONTENT\", " >> $CONTENT.json
    echo "  \"version\": \"$version\", " >> $CONTENT.json
    echo "  \"source_path\": { " >> $CONTENT.json

    arches=("amd64")
    oses=("linux" "darwin")
    archcomma=""
    for arch in "${arches[@]}"; do
        echo "    $archcomma \"$arch\": { " >> $CONTENT.json

        oscomma=""
        for os in "${oses[@]}"; do
            path="$CONTENT/$version/$arch/$os"

            if [[ "$os" == "windows" ]] ; then
                ext=".exe"
            else
                ext=""
            fi

            mkdir -p "rebar-catalog/$path"
            [[ -f  bin/$os/$arch/$cmd${ext} ]] || continue
            cp "bin/$os/$arch/$cmd${ext}" "rebar-catalog/$path"

            echo "      $oscomma \"$os\": \"$path/$cmd${ext}\"" >> $CONTENT.json
            oscomma=","
        done
        echo "    }" >> $CONTENT.json
        archcomma=","
    done
    echo "  }" >> $CONTENT.json
    echo "}" >> $CONTENT.json

    echo "Updating $CONTENT"
    curl -X PUT -T $CONTENT.json https://qww9e4paf1.execute-api.us-west-2.amazonaws.com/main/support/plugin/$CONTENT?token=$TOKEN
done

