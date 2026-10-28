import * as React from "react";
import { cn } from "../../lib/utils";
export function Input({
  className,
  type,
  ...props
}: React.ComponentProps<"input">) {
  return (
    <input
      data-slot="input"
      type={type}
      className={cn("input", className)}
      {...props}
    />
  );
}
