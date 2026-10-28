import type { ComponentProps } from "react";
import { cn } from "../../lib/utils";
export function Table({ className, ...props }: ComponentProps<"table">) {
  return (
    <div className="table-scroll">
      <table className={cn("data-table", className)} {...props} />
    </div>
  );
}
export function TableHeader(props: ComponentProps<"thead">) {
  return <thead {...props} />;
}
export function TableBody(props: ComponentProps<"tbody">) {
  return <tbody {...props} />;
}
export function TableRow(props: ComponentProps<"tr">) {
  return <tr {...props} />;
}
export function TableHead(props: ComponentProps<"th">) {
  return <th {...props} />;
}
export function TableCell(props: ComponentProps<"td">) {
  return <td {...props} />;
}
