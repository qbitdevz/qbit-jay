#!/usr/bin/env bash
# Close only the prior server-confirmed binding; Go owns validation and deadlines.
# qbit-jay reads QBIT_JAY_* first and ENGRAM_* as a fallback (an empty value falls
# back). Builtins only.
for _qbit_jay_name in PORT SOCKET URL DATA_DIR NUDGE_COOLDOWN_SECS; do
  _qbit_jay_var="QBIT_JAY_${_qbit_jay_name}"
  if [ -n "${!_qbit_jay_var:-}" ]; then
    printf -v "ENGRAM_${_qbit_jay_name}" '%s' "${!_qbit_jay_var}"
  fi
done
unset _qbit_jay_name _qbit_jay_var


qbit-jay hook codex-session-end >/dev/null 2>&1 || true
exit 0
