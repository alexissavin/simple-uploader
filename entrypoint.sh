#!/bin/sh


mkdir -p /var/html/simple_uploader/data
chown -R goapp:goapp /var/html/simple_uploader/data
chmod -R 770 /var/html/simple_uploader

if [ -d '/etc/simple_uploader' ] && [ -d '/etc/simple_uploader/tokens' ]; then
  mkdir -p /tmp/simple_uploader
  cp -r /etc/simple_uploader/tokens /tmp/simple_uploader/
  chown -R goapp:goapp /tmp/simple_uploader/tokens
  chmod -R 750 /tmp/simple_uploader/tokens
fi

exec runuser -u goapp "/usr/local/bin/app" -- "-tokens" "/tmp/simple_uploader/tokens/tokens" "$@"