export type Transaction = {
  id: string;
  account_id: string;
  bank_id: number;
  external_id?: string;
  amount: number;
  currency: number;
  operation_amount?: number;
  operation_currency?: number;
  balance?: number;
  cashback_amount?: number;
  commission_rate?: number;
  mcc: number;
  category?: string;
  description: string;
  comment?: string;
  hold?: boolean;
  occurred_at: string;
};

export type Account = {
  id: string;
  userId: string;
  bank: number;
  external_id?: string;
  monobank_account_id?: string;
  name: string;
  currency: number;
  balance: number; // own money, negative = owed
  credit_limit?: number; // 0 or missing = debit
};

export type ChartMode = "all" | "quarters" | "months" | "days";

export type Role = "regular" | "admin";

export type User = {
  id: string;
  email: string;
  role: Role;
};

/** Sign-up allow-list entry (admin settings) */
export type AllowedEmail = {
  id: string;
  email: string;
  role: Role;
  registered: boolean;
  registered_at?: string;
  created_at: string;
};
