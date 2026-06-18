import { create } from 'zustand';

export interface UpdateInfo {
  version: string;
  releaseNotes?: string;
}

export interface ProgressInfo {
  percent: number;
  transferred: number;
  total: number;
  bytesPerSecond: number;
}

export interface UpdateError {
  message: string;
  platform?: string;
}

type UpdateStatus = 'idle' | 'available' | 'downloading' | 'downloaded' | 'error' | 'stalled';

export const POSTPONE_MS = 60 * 60 * 1000; // default 1 giờ

export const POSTPONE_OPTIONS: { label: string; ms: number }[] = [
  { label: '15 phút',  ms: 15 * 60 * 1000 },
  { label: '1 giờ',   ms: 60 * 60 * 1000 },
  { label: '3 giờ',   ms: 3 * 60 * 60 * 1000 },
  { label: '6 giờ',   ms: 6 * 60 * 60 * 1000 },
  { label: '12 giờ',  ms: 12 * 60 * 60 * 1000 },
];

interface UpdateStore {
  status: UpdateStatus;
  updateInfo: UpdateInfo | null;
  progress: ProgressInfo | null;
  error: UpdateError | null;
  dismissed: boolean;            // user đang hoãn popup
  postponedUntil: number | null; // timestamp khi hết hoãn
  platform: string;              // 'darwin' | 'win32' | 'linux'

  setStatus: (status: UpdateStatus) => void;
  setUpdateInfo: (info: UpdateInfo | null) => void;
  setProgress: (progress: ProgressInfo | null) => void;
  setError: (error: UpdateError | null) => void;
  setDismissed: (dismissed: boolean) => void;
  setPostponedUntil: (ts: number | null) => void;
  setPlatform: (platform: string) => void;

  /** Hoãn notification durationMs ms rồi tự hiện lại */
  postpone: (durationMs?: number) => void;

  /** Có bản update cần hành động (chưa tải xong) — hiện nút trên TopBar */
  hasActionableUpdate: () => boolean;
}

export const useUpdateStore = create<UpdateStore>((set, get) => ({
  status: 'idle',
  updateInfo: null,
  progress: null,
  error: null,
  dismissed: false,
  postponedUntil: null,
  platform: (window as any).electronAPI?.platform || 'win32',

  setStatus: (status) => set({ status }),
  setUpdateInfo: (updateInfo) => set({ updateInfo }),
  setProgress: (progress) => set({ progress }),
  setError: (error) => set({ error }),
  setDismissed: (dismissed) => set({ dismissed }),
  setPostponedUntil: (postponedUntil) => set({ postponedUntil }),
  setPlatform: (platform) => set({ platform }),

  postpone: (durationMs = POSTPONE_MS) => {
    set({ dismissed: true, postponedUntil: Date.now() + durationMs });
  },

  hasActionableUpdate: () => {
    const { status, updateInfo } = get();
    return !!updateInfo && ['available', 'error', 'stalled', 'downloading'].includes(status);
  },
}));

