import { onBeforeUnmount, onMounted, ref, useTemplateRef } from "vue";

/**
 * Measures an element's rendered width with a ResizeObserver and exposes it
 * as a reactive ref. Bind the element in the template with a plain string
 * ref matching `key` — the Vue 3.5 useTemplateRef pattern, e.g.
 *
 *   const { width } = useElementWidth("chartEl");
 *   <div ref="chartEl">…</div>
 *
 * Falls back to a 600px guess when ResizeObserver is unavailable (jsdom).
 * Width is floored to whole pixels so HTML overlays positioned from it stay
 * on integer coordinates.
 */
export function useElementWidth(key: string) {
  const el = useTemplateRef<HTMLElement>(key);
  const width = ref(600);

  let ro: ResizeObserver | undefined;
  onMounted(() => {
    const node = el.value;
    // Property probe on globalThis: never throws in jsdom, where the global
    // may not exist at all (a bare `ResizeObserver` reference would).
    const RO = globalThis.ResizeObserver;
    if (!node || !RO) return;
    ro = new RO((entries) => {
      const w = entries.at(-1)?.contentRect.width;
      if (w !== undefined) width.value = Math.max(0, Math.floor(w));
    });
    ro.observe(node);
  });
  onBeforeUnmount(() => ro?.disconnect());

  return { width };
}
