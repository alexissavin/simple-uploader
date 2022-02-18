#!/bin/sh

mkdir -p /var/html/simple_uploader/data
chown -R goapp:goapp /var/html/simple_uploader/data
chmod -R 775 /var/html/simple_uploader

if [ -d '/etc/simple_uploader' ] && [ -d '/etc/simple_uploader/tokens' ]; then
  mkdir -p /tmp/simple_uploader
  cp -r /etc/simple_uploader/tokens /tmp/simple_uploader/
  chown -R goapp:goapp /tmp/simple_uploader
  chmod -R 750 /tmp/simple_uploader/tokens
  if [ `ls -1 '/tmp/simple_uploader/tokens'| wc -l` -gt 0 ]; then
    chmod 640 /tmp/simple_uploader/tokens/*
  fi
fi

exec runuser -u goapp "/usr/local/bin/app" -- "-tokens" "/tmp/simple_uploader/tokens/tokens" "$@"