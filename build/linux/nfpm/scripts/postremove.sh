#!/bin/sh
set -e

# Unregister the custom URL scheme handler
if [ -x /usr/bin/xdg-mime ]; then
    xdg-mime uninstall x-scheme-handler/super 2>/dev/null || true
fi

# Update desktop database
if [ -x /usr/bin/update-desktop-database ]; then
    update-desktop-database /usr/share/applications
fi
