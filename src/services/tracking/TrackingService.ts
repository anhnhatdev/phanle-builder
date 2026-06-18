/**
 * TrackingService — Gửi dữ liệu tracking page lên API phanleapp.com.
 *
 * - Chỉ hoạt động khi build production (GitHub Actions / NODE_ENV=production).
 * - Cache local: mỗi ngày chỉ push 1 lần.
 * - Gửi pageId (zalo_id), machineId, lastTrackedAt lên Google Sheets.
 * - Không gửi các field nhạy cảm khác (name, phone,...).
 *
 * API: POST https://phanleapp.com/api/tracking/page
 * Rate limit: 10 req/giờ/IP — trả về 429 nếu vượt quá.
 */

import { IS_DEV_BUILD } from '../../configs/BuildConfig';
import DatabaseService from '../database/DatabaseService';
import Logger from '../../utils/Logger';
import * as path from 'path';
import * as fs from 'fs';
import { app } from 'electron';
import { v4 as uuidv4 } from 'uuid';
import axios from 'axios';

// ─── Types ──────────────────────────────────────────────────────────────────

interface TrackingPayload {
  pageId: string;
  machineId: string;
  lastTrackedAt: string;
}

interface TrackingCache {
  machineId: string;
  lastTrackedDate: string;       // ISO date string "YYYY-MM-DD"
  lastTrackedAt: string;         // ISO datetime string
}

interface TrackingApiResponse {
  success: boolean;
  message: string;
  upserted?: number;
  errors?: Array<{ pageId: string; error: string }>;
  retryAfterSec?: number;
}

// ─── Constants ──────────────────────────────────────────────────────────────

const API_URL = 'https://phanleapp.com/api/tracking/page';
const CACHE_FILENAME = 'tracking-cache.json';
const CHECK_INTERVAL_MS = 60 * 60 * 1000; // 60 phút

// ─── TrackingService ────────────────────────────────────────────────────────

class TrackingService {
  private static instance: TrackingService;
  private cache: TrackingCache | null = null;
  private cachePath = '';
  private timer: ReturnType<typeof setInterval> | null = null;
  private running = false;

  public static getInstance(): TrackingService {
    if (!TrackingService.instance) {
      TrackingService.instance = new TrackingService();
    }
    return TrackingService.instance;
  }

  /**
   * Khởi động tracking service.
   * Chỉ hoạt động trong production build.
   */
  public start(): void {
    // ── Guard: chỉ chạy trong production ────────────────────────────────
    if (IS_DEV_BUILD) {
      Logger.log('[TrackingService] 🔇 Bỏ qua — đang chạy ở môi trường development');
      return;
    }

    if (this.running) {
      Logger.log('[TrackingService] ⚠️ Đã khởi động rồi, bỏ qua');
      return;
    }

    this.running = true;

    try {
      this.cachePath = path.join(app.getPath('userData'), CACHE_FILENAME);
      this.loadCache();

      Logger.log(`[TrackingService] ✅ Khởi động — machineId=${this.cache?.machineId?.slice(0, 8)}... lastTracked=${this.cache?.lastTrackedDate || 'never'}`);

      // Chạy lần đầu sau 10s (đợi DB sẵn sàng)
      setTimeout(() => this.tick(), 10_000);

      // Lặp mỗi 60 phút
      this.timer = setInterval(() => this.tick(), CHECK_INTERVAL_MS);
    } catch (err: any) {
      Logger.error(`[TrackingService] ❌ Lỗi khởi động: ${err.message}`);
    }
  }

  /**
   * Dừng tracking service.
   */
  public stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.running = false;
    Logger.log('[TrackingService] 🔇 Đã dừng');
  }

  // ── Core logic ──────────────────────────────────────────────────────────

  private async tick(): Promise<void> {
    try {
      // Kiểm tra xem hôm nay đã gửi chưa
      const today = new Date().toISOString().slice(0, 10); // "YYYY-MM-DD"
      if (this.cache?.lastTrackedDate === today) {
        return; // Hôm nay đã gửi rồi
      }

      Logger.log('[TrackingService] 📡 Bắt đầu gửi tracking hôm nay...');
      await this.sendTracking();

      // Cập nhật cache sau khi gửi thành công
      this.cache = {
        machineId: this.cache?.machineId || this.generateMachineId(),
        lastTrackedDate: today,
        lastTrackedAt: new Date().toISOString(),
      };
      this.saveCache();

      Logger.log('[TrackingService] ✅ Tracking hôm nay đã gửi thành công');
    } catch (err: any) {
      Logger.warn(`[TrackingService] ⚠️ Gửi tracking thất bại: ${err.message}`);
      // Không crash app — thử lại vào lần tick sau (60 phút)
    }
  }

  /**
   * Thu thập dữ liệu accounts (Zalo + Facebook) và gửi lên API.
   */
  private async sendTracking(): Promise<void> {
    const machineId = this.cache?.machineId || this.generateMachineId();

    // ── Lấy Zalo accounts ──────────────────────────────────────────────
    let zaloAccounts: Array<{ zalo_id: string }> = [];
    try {
      zaloAccounts = DatabaseService.getInstance().getAccounts();
    } catch (dbErr: any) {
      Logger.warn(`[TrackingService] ⚠️ Không thể đọc Zalo accounts từ DB: ${dbErr.message}`);
    }

    // ── Lấy Facebook accounts ──────────────────────────────────────────
    let fbAccounts: Array<{ facebook_id: string }> = [];
    try {
      const allFb = DatabaseService.getInstance().getFBAccounts() || [];
      fbAccounts = allFb
        .filter((fb: any) => fb.facebook_id && fb.status !== 'disconnected')
        .map((fb: any) => ({ facebook_id: fb.facebook_id }));
    } catch (dbErr: any) {
      Logger.warn(`[TrackingService] ⚠️ Không thể đọc Facebook accounts từ DB: ${dbErr.message}`);
    }

    // ── Không có account nào ───────────────────────────────────────────
    if (zaloAccounts.length === 0 && fbAccounts.length === 0) {
      Logger.log('[TrackingService] ℹ️ Không có account nào — gửi machineId không');
      const payload: TrackingPayload[] = [
        {
          pageId: `machine:${machineId.slice(0, 12)}`,
          machineId,
          lastTrackedAt: this.formatDate(new Date()),
        },
      ];
      await this.postTracking(payload);
      return;
    }

    // ── Build payload: mỗi account 1 entry (Zalo + Facebook) ──────────
    const now = new Date();
    const lastTrackedAt = this.formatDate(now);
    const payload: TrackingPayload[] = [];

    // Zalo accounts → pageId = zalo_id
    for (const acc of zaloAccounts) {
      payload.push({ pageId: acc.zalo_id, machineId, lastTrackedAt });
    }

    // Facebook accounts → pageId = facebook_id
    for (const acc of fbAccounts) {
      payload.push({ pageId: acc.facebook_id, machineId, lastTrackedAt });
    }

    Logger.log(`[TrackingService] 📤 Gửi ${payload.length} page(s) lên API (Zalo=${zaloAccounts.length}, FB=${fbAccounts.length})...`);

    await this.postTracking(payload);
  }

  /**
   * POST dữ liệu lên API tracking.
   */
  private async postTracking(payload: TrackingPayload[]): Promise<void> {
    const response = await axios.post<TrackingApiResponse>(
      API_URL,
      { pages: payload },
      {
        headers: { 'Content-Type': 'application/json' },
        timeout: 15_000, // 15 giây timeout
      },
    );

    const data = response.data;

    if (data.success) {
      Logger.log(`[TrackingService] ✅ API: ${data.message} (upserted=${data.upserted})`);
    } else if (data.errors && data.errors.length > 0) {
      Logger.warn(`[TrackingService] ⚠️ API partial error: ${data.message}`);
      for (const err of data.errors) {
        Logger.warn(`[TrackingService]   → pageId=${err.pageId}: ${err.error}`);
      }
    } else {
      Logger.warn(`[TrackingService] ⚠️ API trả về lỗi: ${data.message}`);
    }
  }

  // ── Cache management ────────────────────────────────────────────────────

  private loadCache(): void {
    try {
      if (fs.existsSync(this.cachePath)) {
        const raw = fs.readFileSync(this.cachePath, 'utf-8');
        this.cache = JSON.parse(raw) as TrackingCache;
        Logger.log(`[TrackingService] 📂 Đã load cache: lastTrackedDate=${this.cache.lastTrackedDate}`);
      }
    } catch (err: any) {
      Logger.warn(`[TrackingService] ⚠️ Lỗi đọc cache: ${err.message} — sẽ tạo mới`);
    }
  }

  private saveCache(): void {
    try {
      const dir = path.dirname(this.cachePath);
      if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
      }
      fs.writeFileSync(this.cachePath, JSON.stringify(this.cache, null, 2), 'utf-8');
    } catch (err: any) {
      Logger.warn(`[TrackingService] ⚠️ Lỗi ghi cache: ${err.message}`);
    }
  }

  /**
   * Sinh hoặc lấy machineId cố định cho máy này.
   * Lưu trong cache để không đổi mỗi lần chạy app.
   */
  private generateMachineId(): string {
    const id = uuidv4();
    Logger.log(`[TrackingService] 🆔 Đã tạo machineId mới: ${id.slice(0, 8)}...`);
    return id;
  }

  /**
   * Format date theo định dạng DD/MM/YYYY HH:mm (Google Sheets format).
   */
  private formatDate(date: Date): string {
    const dd = String(date.getDate()).padStart(2, '0');
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const yyyy = date.getFullYear();
    const hh = String(date.getHours()).padStart(2, '0');
    const min = String(date.getMinutes()).padStart(2, '0');
    return `${dd}/${mm}/${yyyy} ${hh}:${min}`;
  }
}

export default TrackingService;
