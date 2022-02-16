#!/bin/bash

chown -R goapp:goapp /etc/simple_uploader/tokens
chmod -R 750 /etc/simple_uploader/tokens
mkdir -p /var/html/simple_uploader/data
chown -R goapp:goapp /var/html/simple_uploader/data
chmod -R 770 /var/html/simple_uploader

exec runuser -u goapp /usr/local/bin/app "$@"