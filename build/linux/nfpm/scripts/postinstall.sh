#!/bin/sh
set -e

# Update desktop database to register the MIME type
if [ -x /usr/bin/update-desktop-database ]; then
    update-desktop-database /usr/share/applications
fi

# Register the custom URL scheme handler
xdg-mime default SUPER.desktop x-scheme-handler/super 2>/dev/null || true

echo "SUPER deep link registered: super://"
