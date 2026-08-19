// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as React from "react";
import {ChevronDown, ChevronRight, ChevronsUpDown, ChevronUp} from "lucide-react";
import i18next from "i18next";

import {cn} from "@/lib/utils";
import {Button} from "@/components/ui/button";
import {Spinner} from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export type SortOrder = "ascend" | "descend" | undefined;

export interface Column<T> {
  title: React.ReactNode;
  key: string;
  dataIndex?: string;
  /** A CSS width, e.g. "120px". Columns without one share the leftover space. */
  width?: string;
  className?: string;
  /**
   * A comparator sorts the rows in the browser. `true` means the server sorts,
   * and the click is forwarded to `onSort` instead.
   */
  sorter?: ((a: T, b: T) => number) | true;
  render?: (value: any, record: T, index: number) => React.ReactNode;
}

export interface ServerPagination {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number, pageSize: number) => void;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[] | null | undefined;
  rowKey: (record: T, index: number) => string;
  loading?: boolean;
  /** Heading shown above the table, next to `toolbar`. */
  title?: React.ReactNode;
  toolbar?: React.ReactNode;
  /** Rows per page when the browser paginates. 0 shows everything at once. */
  pageSize?: number;
  /** Set when the server paginates: `data` is then already the current page. */
  serverPagination?: ServerPagination;
  onSort?: (field: string, order: SortOrder) => void;
  emptyText?: React.ReactNode;
  expandedRowRender?: (record: T) => React.ReactNode;
  rowClassName?: (record: T, index: number) => string | undefined;
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

function getValue<T>(record: T, column: Column<T>) {
  if (!column.dataIndex) {
    return undefined;
  }
  return (record as Record<string, any>)[column.dataIndex];
}

export function DataTable<T>({
  columns,
  data,
  rowKey,
  loading = false,
  title,
  toolbar,
  pageSize = 10,
  serverPagination,
  onSort,
  emptyText,
  expandedRowRender,
  rowClassName,
}: DataTableProps<T>) {
  const [sortKey, setSortKey] = React.useState<string>("");
  const [sortOrder, setSortOrder] = React.useState<SortOrder>(undefined);
  const [page, setPage] = React.useState(1);
  const [localPageSize, setLocalPageSize] = React.useState(pageSize);
  const [expanded, setExpanded] = React.useState<Record<string, boolean>>({});

  // Memoised so that a page passing `data={undefined}` does not hand the sort
  // below a fresh array on every render.
  const rows = React.useMemo(() => data ?? [], [data]);

  // A shrinking result set (a delete, a new filter) can leave the viewer on a
  // page that no longer exists, which would render as an empty table.
  const localPageCount = localPageSize > 0 ? Math.max(1, Math.ceil(rows.length / localPageSize)) : 1;
  React.useEffect(() => {
    if (!serverPagination && page > localPageCount) {
      setPage(localPageCount);
    }
  }, [localPageCount, page, serverPagination]);

  const toggleSort = (column: Column<T>) => {
    if (!column.sorter) {
      return;
    }

    const nextOrder: SortOrder =
      sortKey !== column.key ? "ascend" : sortOrder === "ascend" ? "descend" : undefined;
    setSortKey(nextOrder === undefined ? "" : column.key);
    setSortOrder(nextOrder);

    if (column.sorter === true) {
      onSort?.(column.dataIndex ?? column.key, nextOrder);
    }
  };

  const sorted = React.useMemo(() => {
    const column = columns.find(item => item.key === sortKey);
    if (!column || typeof column.sorter !== "function" || sortOrder === undefined) {
      return rows;
    }

    const comparator = column.sorter;
    const copy = [...rows];
    copy.sort((a, b) => {
      const result = comparator(a, b);
      return sortOrder === "ascend" ? result : -result;
    });
    return copy;
  }, [columns, rows, sortKey, sortOrder]);

  const pageRows = React.useMemo(() => {
    if (serverPagination || localPageSize <= 0) {
      return sorted;
    }
    const start = (page - 1) * localPageSize;
    return sorted.slice(start, start + localPageSize);
  }, [localPageSize, page, serverPagination, sorted]);

  const currentPage = serverPagination ? serverPagination.page : page;
  const currentPageSize = serverPagination ? serverPagination.pageSize : localPageSize;
  const total = serverPagination ? serverPagination.total : sorted.length;
  const pageCount = currentPageSize > 0 ? Math.max(1, Math.ceil(total / currentPageSize)) : 1;

  const goTo = (nextPage: number, nextPageSize = currentPageSize) => {
    if (serverPagination) {
      serverPagination.onChange(nextPage, nextPageSize);
      return;
    }
    setPage(nextPage);
    setLocalPageSize(nextPageSize);
  };

  const showPager = currentPageSize > 0 && (total > currentPageSize || currentPage > 1);
  const columnCount = columns.length + (expandedRowRender ? 1 : 0);

  return (
    <div className="rounded-xl border bg-card shadow-sm">
      {(title || toolbar) && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-2.5">
          <div className="flex items-center gap-3 text-sm font-semibold">
            {title}
            {loading ? <Spinner className="text-muted-foreground" /> : null}
          </div>
          <div className="flex items-center gap-2">{toolbar}</div>
        </div>
      )}
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/40 hover:bg-muted/40">
            {expandedRowRender ? <TableHead className="w-8" /> : null}
            {columns.map(column => (
              <TableHead
                key={column.key}
                style={column.width ? {width: column.width, minWidth: column.width} : undefined}
                className={cn(column.sorter && "cursor-pointer select-none", column.className)}
                onClick={() => toggleSort(column)}
              >
                <span className="inline-flex items-center gap-1">
                  {column.title}
                  {column.sorter ? (
                    sortKey === column.key && sortOrder === "ascend" ? (
                      <ChevronUp className="h-3 w-3" />
                    ) : sortKey === column.key && sortOrder === "descend" ? (
                      <ChevronDown className="h-3 w-3" />
                    ) : (
                      <ChevronsUpDown className="h-3 w-3 opacity-40" />
                    )
                  ) : null}
                </span>
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageRows.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columnCount} className="h-24 text-center text-muted-foreground">
                {loading ? <Spinner className="mx-auto" /> : (emptyText ?? "No data")}
              </TableCell>
            </TableRow>
          ) : (
            pageRows.map((record, index) => {
              const key = rowKey(record, index);
              const isOpen = expanded[key] === true;
              return (
                <React.Fragment key={key}>
                  <TableRow className={rowClassName?.(record, index)}>
                    {expandedRowRender ? (
                      <TableCell className="w-8 align-top">
                        <button
                          type="button"
                          aria-label={isOpen ? "Collapse row" : "Expand row"}
                          className="text-muted-foreground hover:text-foreground"
                          onClick={() => setExpanded(current => ({...current, [key]: !isOpen}))}
                        >
                          {isOpen ? (
                            <ChevronDown className="h-4 w-4" />
                          ) : (
                            <ChevronRight className="h-4 w-4" />
                          )}
                        </button>
                      </TableCell>
                    ) : null}
                    {columns.map(column => (
                      <TableCell
                        key={column.key}
                        style={column.width ? {width: column.width, minWidth: column.width} : undefined}
                        className={column.className}
                      >
                        {column.render
                          ? column.render(getValue(record, column), record, index)
                          : (getValue(record, column) as React.ReactNode)}
                      </TableCell>
                    ))}
                  </TableRow>
                  {isOpen && expandedRowRender ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={columnCount} className="bg-muted/30 p-0">
                        {expandedRowRender(record)}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </React.Fragment>
              );
            })
          )}
        </TableBody>
      </Table>
      {showPager && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2 text-sm text-muted-foreground">
          <span>{i18next.t("general:{total} in total").replace("{total}", String(total))}</span>
          <div className="flex items-center gap-2">
            <select
              className="h-8 rounded-md border border-input bg-transparent px-2 text-sm"
              value={currentPageSize}
              onChange={event => goTo(1, Number(event.target.value))}
            >
              {PAGE_SIZE_OPTIONS.map(option => (
                <option key={option} value={option}>
                  {option} / page
                </option>
              ))}
            </select>
            <Button
              variant="outline"
              size="sm"
              disabled={currentPage <= 1}
              onClick={() => goTo(currentPage - 1)}
            >
              {"<"}
            </Button>
            <span className="tabular-nums">
              {currentPage} / {pageCount}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={currentPage >= pageCount}
              onClick={() => goTo(currentPage + 1)}
            >
              {">"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
