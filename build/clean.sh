#!/usr/bin/env bash 

# Delete version tags.
git fetch
git push origin --delete $(git tag -l)
git tag -d $(git tag -l)

rm -rf modh*

git add .
git commit -m "Clean"

# Start with a clean history.
git checkout --orphan new-master master
git commit -m "Init"
git branch -M new-master master
git push -f

