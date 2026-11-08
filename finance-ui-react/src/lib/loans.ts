export type Loan = {
  id: string;
  name: string;
  bank: number;
  currency: number;
  principal: number; // minor units
  payment: number; // regular monthly payment, minor units
  last_payment: number; // the final payment, minor units
  payments_total: number;
  payments_paid: number;
  first_due: string; // YYYY-MM-DD, any day of the first payment's month
};

export type LoanPayment = { number: number; due: Date; amount: number; overdue: boolean; left: number };

/** Payment k (1-based) is due on the last day of the month first_due + k - 1 */
export function dueDate(loan: Loan, k: number): Date {
  const [year, month] = loan.first_due.split("-").map(Number);
  return new Date(year, month - 1 + k, 0, 23, 59, 59); // day 0 of the next month = last day of this one
}

/** The payments still to make, with the debt left after each one */
export function remainingPayments(loan: Loan, now = new Date()): LoanPayment[] {
  const rows: Omit<LoanPayment, "left">[] = [];
  for (let k = loan.payments_paid + 1; k <= loan.payments_total; k += 1) {
    const due = dueDate(loan, k);
    rows.push({ number: k, due, amount: k === loan.payments_total ? loan.last_payment : loan.payment, overdue: due < now });
  }
  let left = rows.reduce((sum, r) => sum + r.amount, 0);
  return rows.map((r) => {
    left -= r.amount;
    return { ...r, left };
  });
}

/** What is still to pay on the loan, minor units */
export const owedOn = (loan: Loan) => remainingPayments(loan).reduce((sum, p) => sum + p.amount, 0);

/** The loan is assumed taken on the last day of the month before the first payment's month */
export function takenOn(loan: Loan): Date {
  const [year, month] = loan.first_due.split("-").map(Number);
  return new Date(year, month - 1, 0);
}

/** The payments already made, with their due dates (minor units) */
export function paidPayments(loan: Loan): { due: Date; amount: number }[] {
  return Array.from({ length: loan.payments_paid }, (_, i) => ({
    due: dueDate(loan, i + 1),
    amount: i + 1 === loan.payments_total ? loan.last_payment : loan.payment,
  }));
}

/** Everything the loan costs from the first payment to the last, minor units */
export const totalToRepay = (loan: Loan) => loan.payment * (loan.payments_total - 1) + loan.last_payment;
