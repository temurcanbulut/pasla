# Distribution status

The GitHub Release workflow builds Windows x64/ARM64 and Linux x64/ARM64 files
only after successful CI for the exact source commit. Existing immutable releases
are not replaced. Install from the linked GitHub release and verify SHA256.

Windows Authenticode signing requires an owner-controlled signing identity or a
trusted signing service; a self-signed certificate would not establish publisher
trust. No certificate was invented, purchased or used during this review.

Scoop/winget submission must target the new tested version and real installer
hashes. No package-manager listing has been claimed or submitted from an unsigned,
untested placeholder. macOS/Homebrew support also requires a macOS build and
native tests; the current Windows/Linux release is not relabeled as a Mac build.

These are external distribution prerequisites, not hidden completed features.
Do not add credentials to the repository. Keep any future signing/publishing
permission separate from ordinary build/test jobs.
