import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
export function date(value?: string) {
  if (!value) return "Sin datos";
  const d = new Date(value);
  return Number.isNaN(+d)
    ? "Sin datos"
    : new Intl.DateTimeFormat("es", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(d);
}
export function bytes(n?: number | null) {
  if (n == null) return "Sin datos";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < 4) {
    n /= 1024;
    i++;
  }
  return `${new Intl.NumberFormat("es", { maximumFractionDigits: 1 }).format(n)} ${units[i]}`;
}
