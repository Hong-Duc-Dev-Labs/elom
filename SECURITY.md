# Security

Elom deletes files and changes a few Windows settings. This page states what it
may touch, what it never touches, and how the privileged parts work.

## Protected locations

`internal/core/safety` refuses a path unless it is inside one of the folders
cleanup may work in, and always refuses:

- volume roots such as `C:\`;
- `%SystemRoot%`, `Program Files`, `Program Files (x86)`, `%ProgramData%\Microsoft`,
  `%ProgramData%\Package Cache`;
- the profile root and the personal folders inside it (Desktop, Documents,
  Downloads, Pictures, Videos, Music, Saved Games, OneDrive*);
- UNC paths, alternate data streams (`file:stream`) and any path whose ancestors
  include a junction, symbolic link or cloud placeholder.

The guard runs again at deletion time: a path approved by a scan is not trusted
input. The protection list (Settings › Protection list) additionally blocks
paths, and cleanup refuses to run at all when that list or the operation log
cannot be read.

## What is deleted, and how

- Cleanup defaults to the Recycle Bin through MyGo's `Shell.TrashItem`, which
  fails instead of deleting when the bin cannot take an item.
- Permanent deletion is opt-in and applies only to the `system-temp` category.
- Every deletion writes one append-only record to
  `%APPDATA%\Elom\operations.log` (run id, time, operation, path, bytes,
  outcome). Nothing is uploaded; the log lives in the user's own profile.

## Privilege boundary

The user interface never runs elevated. Maintenance operations that need
administrator rights run in a second invocation of the same executable:

1. the UI writes a job file to `%TEMP%` with a closed list of operations;
2. `ShellExecuteW("runas", …)` raises one UAC prompt;
3. the elevated process parses the job strictly (version 1, no unknown fields,
   no unknown operations, at most 64 KiB), re-validates every operation and
   refuses to run any operation marked as privileged when its own token is not
   elevated;
4. results are written next to the job and read back by the UI.

The job file is untrusted input: any process of the same user can create one.

## Reporting

Open an issue with the steps, the affected operation, and the relevant lines of
`operations.log` with personal paths redacted. Do not send secrets.
