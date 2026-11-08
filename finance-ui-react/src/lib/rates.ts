// National Bank of Ukraine official rates (public, open to browsers): UAH per 1 unit of the currency
const NBU = "https://bank.gov.ua/NBUStatService/v1/statdirectory/exchange";
const cache = new Map<string, Promise<number | null>>();

const ymd = (d: Date) => `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, "0")}${String(d.getDate()).padStart(2, "0")}`;

/** The NBU rate of a currency (ISO letters) on a day; no day: the latest. null when the NBU has none. */
export function nbuRate(code: string, day?: Date): Promise<number | null> {
  const key = `${code}:${day ? ymd(day) : "latest"}`;
  let rate = cache.get(key);
  if (!rate) {
    const url = `${NBU}?valcode=${code}${day ? `&date=${ymd(day)}` : ""}&json`;
    rate = fetch(url)
      .then((r) => (r.ok ? r.json() : []))
      .then((rows: { rate: number }[]) => rows[0]?.rate ?? null)
      .catch(() => null);
    cache.set(key, rate);
  }
  return rate;
}
