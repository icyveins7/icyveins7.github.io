#!/bin/bash

# Intended to be run in this directory.

OLD_ROOT="$HOME/gitrepos/ghpage-blog/data/blog"
NEW_ROOT="../content/posts"
CONVERTER="./converter"

find "$OLD_ROOT" -type f -name '*.mdx' -print0 |
while IFS= read -r -d '' src; do
    # Path relative to the old blog directory
    rel="${src#$OLD_ROOT/}"

    # Change .mdx -> .md
    rel="${rel%.mdx}.md"

    # Corresponding destination path
    dst="$NEW_ROOT/$rel"

    echo "$src -> $dst"

    "$CONVERTER" "$src" "$dst" || {
        echo "ERROR converting: $src" >&2
        exit 1
    }
done
