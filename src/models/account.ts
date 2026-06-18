/** Tài khoản unified — Zalo + Facebook */
export interface Account {
    id?: number;
    zalo_id: string;
    full_name: string;
    avatar_url: string;
    phone?: string;
    /** 1 = tài khoản Business (trả phí), 0 = cá nhân */
    is_business?: number;
    imei: string;
    user_agent: string;
    cookies: string;
    is_active: number;
    created_at: string;
    last_seen?: string;
    listener_active?: number;
    channel?: string;
    proxy_id?: number | null;
}
