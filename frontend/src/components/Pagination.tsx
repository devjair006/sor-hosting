import { useEffect, useRef } from "react";
import { Button } from "./ui/button";
export function Pagination({
  cursor,
  next,
  onChange,
}: {
  cursor: string;
  next?: string | null;
  onChange: (cursor: string) => void;
}) {
  const previous = useRef<string[]>([]);
  useEffect(() => {
    if (!cursor) previous.current = [];
  }, [cursor]);
  if (!cursor && !next) return null;
  return (
    <nav className="pagination" aria-label="Paginación de resultados">
      <Button
        variant="outline"
        size="sm"
        disabled={!previous.current.length}
        onClick={() => onChange(previous.current.pop() ?? "")}
      >
        Anterior
      </Button>
      <Button
        variant="outline"
        size="sm"
        disabled={!next}
        onClick={() => {
          if (next) {
            previous.current.push(cursor);
            onChange(next);
          }
        }}
      >
        Siguiente
      </Button>
    </nav>
  );
}
