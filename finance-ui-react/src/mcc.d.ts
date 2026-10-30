declare module "mcc" {
  export interface MCCEntry {
    mcc: string;
    edited_description: string;
    combined_description: string;
    usda_description: string;
    irs_description: string;
    irs_reportable: string;
    id: number;
  }

  export function get(mcc: number | string): MCCEntry | undefined;
  export function getByEmoji(emoji: string): MCCEntry | undefined;
  export const all: MCCEntry[];
}
