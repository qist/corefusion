[OPEN] CoreFusion runtime startup debug

## Session
- session_id: root-runtime-startup
- date: 2026-06-29
- scope: investigate how to start/stop and debug `corefusion` when sandboxed runtime lacks required privileges

## Symptom
- `corefusion` main API can sometimes respond, but managed components fail under sandboxed execution.
- `nginx` fails with `chown(..., 999) failed (22: Invalid argument)`.
- `xray` fails with `failed to disable ipv4 rp_filter for all: ... read-only file system`.

## Constraints
- Do not modify business logic before runtime evidence is confirmed.
- Current agent command execution is wrapped by sandbox.
- User expectation: service start/stop should be handled by the agent, not delegated back to the user.

## Initial Hypotheses
1. The agent runtime always wraps commands in a sandbox, so there is no supported switch here to run the service directly as host root.
2. `corefusion` requires host-level privileges such as writable `/proc/sys`, `CAP_NET_ADMIN`, and real `chown`, which the sandbox cannot provide.
3. The currently running `8080/8081` service observed earlier is likely a host-side process outside this sandbox context.
4. A workable debug loop is to let the user run privileged host commands while I prepare exact start/stop/verify commands and interpret logs.
5. If root execution is delegated to the host shell, the remaining failures will shift from permission errors to real service/config issues.

## Next Step
- Confirm agent limitation about non-sandbox execution.
- Provide a root-side start/stop workflow for user-assisted debugging.
- If no non-sandbox execution channel exists, state the limitation clearly and propose the closest workable alternative.
