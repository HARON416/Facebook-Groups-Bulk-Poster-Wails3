#!/bin/sh

# Load the application-specific user namespace permission on Ubuntu 24.04+.
# Other distributions may have an older AppArmor ABI or no AppArmor at all.
if [ -f /proc/sys/kernel/apparmor_restrict_unprivileged_userns ] &&
   [ -d /sys/module/apparmor ] && command -v apparmor_parser >/dev/null 2>&1; then
  if ! apparmor_parser -r /etc/apparmor.d/postika; then
    echo "Failed to load Postika's AppArmor profile; Postika may fail to start." >&2
    exit 1
  fi
fi

# Refresh the installed icon after upgrades as well as first installation.
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q -t /usr/share/icons/hicolor || true
fi

# Update desktop database for .desktop file changes
# This makes the application appear in application menus and registers its capabilities.
if command -v update-desktop-database >/dev/null 2>&1; then
  echo "Updating desktop database..."
  update-desktop-database -q /usr/share/applications
else
  echo "Warning: update-desktop-database command not found. Desktop file may not be immediately recognized." >&2
fi

# Update MIME database for custom URL schemes (x-scheme-handler)
# This ensures the system knows how to handle your custom protocols.
if command -v update-mime-database >/dev/null 2>&1; then
  echo "Updating MIME database..."
  update-mime-database -n /usr/share/mime
else
  echo "Warning: update-mime-database command not found. Custom URL schemes may not be immediately recognized." >&2
fi

exit 0
