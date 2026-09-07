/**
 * Normalizes a rejected service call into the user-facing message. Bound Go
 * errors arrive as "validation: 活动不存在"-style strings — the leading
 * "<kind>: " prefix is the Go error kind and is stripped; anything else is
 * stringified as-is.
 */
export function errorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err);
  return raw.replace(/^\w+:\s*/, "");
}
