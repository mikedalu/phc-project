import { create } from 'zustand';

/**
 * Global command-palette state.
 *
 * A tiny store rather than prop drilling: the trigger lives in the shell and in
 * the dashboard hero, while the dialog renders once at the root of the app. A
 * shared store also guarantees there is exactly one palette, even when a screen
 * is remounted during navigation.
 */
interface CommandPaletteState {
  open: boolean;
  openPalette: () => void;
  closePalette: () => void;
  togglePalette: () => void;
}

export const useCommandPalette = create<CommandPaletteState>((set) => ({
  open: false,
  openPalette: () => set({ open: true }),
  closePalette: () => set({ open: false }),
  togglePalette: () => set((state) => ({ open: !state.open })),
}));
