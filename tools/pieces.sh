#!/usr/bin/env bash

ls cmds | while read cmd ; do
    [[ -d cmds/$cmd ]] || continue
    echo $cmd
done

