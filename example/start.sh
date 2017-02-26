#!/bin/sh

rm -rf /var/log/nginx/access.log /var/log/nginx/error.log
mkfifo /var/log/nginx/access.log
mkfifo /var/log/nginx/error.log

FORMAT='$remote_addr - $remote_user [$time_local] $scheme $host "$request" $request_time $status $body_bytes_sent "$http_referer" "$http_x_forwarded_for" "$http_user_agent"'

sed "s/<<FORMAT>>/$FORMAT/" /etc/nginx/nginx.conf.template >/etc/nginx/nginx.conf || exit $?

if [ -z "$IKEY" ]; then
    echo Must specify an IKEY environment variable!
    exit 1
fi

/ailognginx -ikey $IKEY -format "$FORMAT" -in /var/log/nginx/access.log -role nginx -out - &
/ailogtrace -ikey $IKEY -batch 10 -in /var/log/nginx/error.log -role nginx -out stderr &

exec /usr/sbin/nginx -g "daemon off;"
