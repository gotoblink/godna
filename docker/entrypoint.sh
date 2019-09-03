#!/bin/bash

set -e
set -x

if [ -f .gitconfig.global ]; then
    git config --global include.path "$PWD/.gitconfig.global"
fi

GIT_DESC=`git describe --tags --dirty --always`
GIT_SRC=`git remote get-url origin`

git clone $* /dna-dst
rm -r /dna-dst/*
godna regen /dna-dst

cd /dna-dst
GIT_NEXT_TAG=`godna bumptag ./`
# add updated file eg deleted projects 
git add -u
git commit --allow-empty -m $GIT_SRC" "$GIT_DESC
git tag -a -m $GIT_SRC" "$GIT_DESC $GIT_NEXT_TAG
git push --follow-tags
git push --tags
