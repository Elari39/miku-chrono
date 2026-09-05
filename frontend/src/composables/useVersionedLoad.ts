import { onBeforeUnmount, onMounted, ref, watch, type WatchSource } from "vue";
import { useTimer } from "./useTimer";

/**
 * Run `loader` on mount and whenever the timer version changes (a timer
 * started/stopped anywhere) or an extra trigger fires.
 *
 * Guards against out-of-order responses: each run bumps an internal sequence,
 * and the loader receives an `isCurrent` predicate it must consult before
 * committing any fetched data — a stale response leaves the state untouched.
 */
export function useVersionedLoad(
  loader: (isCurrent: () => boolean) => Promise<void>,
  options: { triggers?: WatchSource[] } = {},
) {
  const { version } = useTimer();
  const loading = ref(false);
  let seq = 0;
  let unmounted = false;

  async function reload(): Promise<void> {
    const my = ++seq;
    loading.value = true;
    try {
      await loader(() => my === seq && !unmounted);
    } finally {
      if (my === seq && !unmounted) loading.value = false;
    }
  }

  onMounted(() => {
    void reload();
    watch([version, ...(options.triggers ?? [])], () => void reload());
  });

  onBeforeUnmount(() => {
    unmounted = true;
    seq++;
  });

  return { loading, reload };
}
