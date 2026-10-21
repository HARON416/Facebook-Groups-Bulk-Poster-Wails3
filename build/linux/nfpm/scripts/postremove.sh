#!/bin/sh

# Unload on removal, keeping the profile active during package upgrades.
case "${1:-}" in
  remove|purge|0)
    if [ -f /proc/sys/kernel/apparmor_restrict_unprivileged_userns ] &&
       command -v apparmor_parser >/dev/null 2>&1; then
      apparmor_parser -R /etc/apparmor.d/postika 2>/dev/null || true
    fi
    ;;
esac
exit 0
