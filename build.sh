#!/bin/sh
set -eu

cd "$(dirname "$0")"
mkdir -p dist/windows-amd64 dist/linux-amd64

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath \
  -ldflags="-s -w -H windowsgui" -o dist/windows-amd64/worklog.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath \
  -ldflags="-s -w" -o dist/linux-amd64/worklog .
cp THIRD_PARTY_NOTICES.md dist/windows-amd64/
cp THIRD_PARTY_NOTICES.md dist/linux-amd64/

printf '构建完成：\n  dist/windows-amd64/worklog.exe\n  dist/linux-amd64/worklog\n'
