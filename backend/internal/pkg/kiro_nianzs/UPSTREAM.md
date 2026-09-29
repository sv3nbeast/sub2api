# nianzs Kiro engine snapshot

This directory is a source-faithful snapshot of backend/internal/pkg/kiro from:

- Repository: https://github.com/nianzs/sub2api
- Commit: d483aefe7c2d1da5139c6f5b011eb6843b1e7dbb
- Commit date: 2026-08-10T03:06:35Z

The Go package name intentionally remains kiro; callers should import this
directory with the alias nianzskiro.

Sub2API compatibility additions live in files prefixed with `sub2api_`. Minimal
hook points in snapshot files may call those additions when the pinned upstream
does not expose an extension point. Refresh the snapshot atomically from the
pinned upstream commit, reapply only the documented hook points, then run the
parity and gateway regression suites.

Documented compatibility hook points:

- EventStream header/exception parsing and redacted diagnostics call helpers in
  `sub2api_eventstream_compat.go`.
- Request construction flattens completed historical tool cycles before KRS.
  KRS can otherwise return only metadata/context usage and no assistant output;
  the active final tool/result pair remains structured.
- Amazon Q and KRS long-context responses may omit metering/messageStop after
  emitting assistant output and a trailing contextUsageEvent. The compatibility
  layer accepts only that ordered, frame-aligned clean-EOF shape; bare EOF,
  metadata-only turns, unfinished tool input, and output after context usage
  stay strict.
- Upstream content-filter stops become Anthropic `stop_reason: "refusal"` with
  `stop_details` (`sub2api_refusal.go`). KRS reports them as a metadataEvent
  with stopReason CONTENT_FILTERED and stopDetails.refusal; the Amazon Q
  endpoint ends the same turn after reasoning with only contextUsageEvent and a
  clean EOF, so that reasoning-only tail is reported as an inferred cyber
  refusal (KRS reports the same requests as CYBER) rather than a completed
  turn. Unsigned thinking cut off by a refusal is
  dropped instead of failing the turn, and a refusal never triggers the native
  tool progress retry.
