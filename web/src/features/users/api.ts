import { apiGet, apiPatch } from '@/lib/api';

/** Admin account shapes — docs/07-API-CONTRACT.md §3, staff administration. */

export interface AdminUser {
  id: string;
  staffNumber: string;
  firstName: string;
  lastName: string;
  email?: string;
  roles: string[];
  isActive: boolean;
  mustChangePassword: boolean;
  lastLoginAt?: string;
  lockedUntil?: string;
  createdAt: string;
}

export interface UserPatch {
  /** Toggles: granted when absent, revoked when present. */
  role?: string;
  isActive?: boolean;
  reason?: string;
}

export async function listUsers(): Promise<AdminUser[]> {
  const { data } = await apiGet<{ users: AdminUser[] }>('/admin/users');
  return data.users;
}

export async function updateUser(id: string, patch: UserPatch): Promise<AdminUser> {
  const { data } = await apiPatch<{ user: AdminUser }>(`/admin/users/${id}`, patch);
  return data.user;
}
