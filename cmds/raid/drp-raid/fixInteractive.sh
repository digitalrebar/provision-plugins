#!/usr/bin/env bash
src="test-data"

go test -timeout 10m "$@" |& tee test.log

readarray -d '' paths < <(find "$src" -depth -type f -name 'volspecs.json' -print0 |sort -t / -k 3 -nz)
for path in "${paths[@]}"; do
    path="${path%/volspecs.json}"
    if ! diff -Nu "$path/expect.log" "$path/actual.log"; then
        read -p "Move $path/actual.log to $path/expect.log? (y/n)" ans
        case $ans in
            y) mv "$path/actual.log" "$path/expect.log";;
            *) true;;
        esac
    fi
    if ! diff -Nu "$path/expect.json" "$path/actual.json"; then
        read -p "Move $path/actual.json to $path/expect.json? (y/n)" ans
        case $ans in
            y) mv "$path/actual.json" "$path/expect.json";;
            *) true;;
        esac
    fi
done
