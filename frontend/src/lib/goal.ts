/**
 * Normalize the daily-goal field into whole minutes within 0–1440 (the
 * backend's validation range). An emptied number input yields "" through
 * `v-model.number` — a JSON decode error if sent to the Go int field — and
 * the HTML min/max attributes do not block typed values, so every unusable
 * input falls back to 0 ("不设目标"), matching the field's own semantics.
 */
export function normalizeDailyGoal(value: number | string | null | undefined): number {
  const n = Math.trunc(Number(value ?? 0));
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.min(n, 1440);
}
