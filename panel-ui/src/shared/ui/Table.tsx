import type { ReactNode } from "react";

export type Column<T> = { header: string; cell: (row: T) => ReactNode; className?: string };

/**
 * A plain table: rows separated by rules, no zebra, no cards. On a narrow
 * screen it scrolls sideways instead of squeezing every cell.
 */
export function Table<T>({ rows, columns, rowKey, caption }: { rows: T[]; columns: Column<T>[]; rowKey: (row: T) => string; caption: string }) {
  return (
    <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <table className="w-full min-w-[36rem] border-collapse text-sm">
        <caption className="sr-only">{caption}</caption>
        <thead>
          <tr className="border-b border-rule text-left text-ink-soft">
            {columns.map((c) => (
              <th key={c.header} scope="col" className={`py-2 pr-4 font-semibold ${c.className ?? ""}`}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={rowKey(r)} className="border-b border-rule-soft align-middle">
              {columns.map((c) => (
                <td key={c.header} className={`py-2.5 pr-4 ${c.className ?? ""}`}>
                  {c.cell(r)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
