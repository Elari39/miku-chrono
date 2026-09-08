// Maps pet states to sprite assets from src/assets/pet/. AI-generated
// transparent PNGs named pet-<file>.png (WebP also accepted) are picked up by
// naming convention, so dropping new art into the folder swaps the pet with
// no code change. Vite stamps hashes / inlines small files at build time.
const modules = import.meta.glob("../assets/pet/pet-*.{png,webp}", {
  eager: true,
  query: "?url",
  import: "default",
}) as Record<string, string>;

export type PetState = "idle" | "running" | "dragging" | "sleeping" | "wave" | "cheer";

/** Asset base name for each runtime state (files are pet-<name>.<ext>). */
const FILE_NAMES: Record<PetState, string> = {
  idle: "idle",
  running: "running",
  dragging: "drag",
  sleeping: "sleep",
  wave: "wave",
  cheer: "cheer",
};

// WebP beats PNG on size when both exist for a state.
const EXT_PRIORITY = ["webp", "png"] as const;

/** Look up a sprite by its raw file key (e.g. "idle" or "running-b"). */
export function petAssetFile(fileKey: string): string {
  for (const ext of EXT_PRIORITY) {
    const url = modules[`../assets/pet/pet-${fileKey}.${ext}`];
    if (url) return url;
  }
  return "";
}

export function petAsset(state: PetState): string {
  return petAssetFile(FILE_NAMES[state]);
}
