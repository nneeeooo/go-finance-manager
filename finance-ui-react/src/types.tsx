export type Transaction = {
  id: string;
  account_id: string;
  bank_id: number;
  amount: number;
  currency: number;
  mcc: number;
  description: string;
  occurred_at: string;
};

export type ChartMode = "all" | "quarters" | "months" | "days";
