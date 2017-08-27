#!/bin/sh

# Clean + build
cd $(dirname $0)
rm -rf ailogtrace/ailogtrace ailognginx/ailognginx ailognginx-* ailogtrace-*
cd ailogtrace
go build || exit $?
cd ../ailognginx
go build || exit $?
cd ..

mv ailognginx/ailognginx ./ailognginx-$1
mv ailogtrace/ailogtrace ./ailogtrace-$1

gpg --armor --detach-sig ailognginx-$1 || exit $1
gpg --armor --detach-sig ailogtrace-$1 || exit $1

gpg --verify ailogtrace-$1.asc ailogtrace-$1
gpg --verify ailognginx-$1.asc ailognginx-$1
