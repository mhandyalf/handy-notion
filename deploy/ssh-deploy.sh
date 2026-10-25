#!/bin/bash
# Forced command for a restricted SSH key; no caller-supplied shell is executed.
set -euo pipefail
if [[ ${SSH_ORIGINAL_COMMAND:-} =~ ^deploy\ ([0-9a-f]{40})$ ]]; then
  exec /usr/local/sbin/handy-notion-auto-update "${BASH_REMATCH[1]}"
fi
echo 'Only deploy <40-character commit SHA> is allowed.' >&2
exit 1