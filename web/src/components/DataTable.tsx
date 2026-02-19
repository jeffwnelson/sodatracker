import type { ReactNode } from "react";

interface Column<T> {
  header: string;
  accessor: (row: T, index: number) => ReactNode;
  className?: string;
  headerClassName?: string;
}

export type { Column };

export function DataTable<T>({
  columns,
  data,
  keyFn,
  emptyMessage = "No data",
  onRowClick,
}: {
  columns: Column<T>[];
  data: T[];
  keyFn: (row: T) => string;
  emptyMessage?: string;
  onRowClick?: (row: T) => void;
}) {
  if (data.length === 0) {
    return (
      <p className="text-sm text-text-tertiary text-center py-8">
        {emptyMessage}
      </p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border-primary">
            {columns.map((col, i) => (
              <th
                key={i}
                className={`pb-3 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary ${col.headerClassName ?? ""}`}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border-primary">
          {data.map((row, rowIndex) => (
            <tr
              key={keyFn(row)}
              className={`transition-colors duration-150 hover:bg-surface-secondary ${onRowClick ? "cursor-pointer" : ""}`}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
            >
              {columns.map((col, colIndex) => (
                <td
                  key={colIndex}
                  className={`py-3 text-text-secondary ${col.className ?? ""}`}
                >
                  {col.accessor(row, rowIndex)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
