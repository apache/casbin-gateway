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
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import {ArrowDown, ArrowUp, ChevronLeft, ChevronRight, ChevronsUpDown, Search} from "lucide-react";
import i18next from "i18next";

import {cn} from "@/lib/utils";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Skeleton} from "@/components/ui/skeleton";
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table";
import {EmptyState, ErrorState} from "@/components/shared/empty-state";

export type SortOrder = "ascend" | "descend" | undefined;

export interface Column<T> {
  key: string;
  title: React.ReactNode;
  /** May be a dotted path. A column without one is a display column (actions). */
  dataIndex?: string;
  render?: (value: any, record: T, index: number) => React.ReactNode;
  /** A CSS width, e.g. "120px". Columns without one share the leftover space. */
  width?: string;
  align?: "left" | "right" | "center";
  /**
   * A comparator sorts the rows in the browser. `true` means the server sorts,
   * and the click is forwarded to `onSort` instead.
   */
  sorter?: ((a: T, b: T) => number) | true;
  ellipsis?: boolean;
  className?: string;
  headerClassName?: string;
}

export interface ServerPagination {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number, pageSize: number) => void;
}

function readPath(row: any, path?: string) {
  if (path === undefined || path === null) {
    return undefined;
  }
  if (!path.includes(".")) {
    return row?.[path];
  }
  return path.split(".").reduce((acc, part) => (acc === null || acc === undefined ? acc : acc[part]), row);
}

function resolveRowKey<T>(rowKey: string | ((record: T, index: number) => string), record: T, index: number) {
  if (typeof rowKey === "function") {
    return rowKey(record, index);
  }
  const value = readPath(record, rowKey);
  return value === undefined || value === null ? String(index) : String(value);
}

function SortIcon({state}: {state: false | "asc" | "desc"}) {
  if (state === "asc") {
    return <ArrowUp className="size-3.5" />;
  }
  if (state === "desc") {
    return <ArrowDown className="size-3.5" />;
  }
  return <ChevronsUpDown className="size-3.5 opacity-40" />;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  dataSource: T[] | null | undefined;
  rowKey?: string | ((record: T, index: number) => string);
  loading?: boolean;
  /** Heading shown on the left of the table header. */
  title?: React.ReactNode;
  description?: React.ReactNode;
  /** Node rendered on the right of the table header. */
  toolbar?: React.ReactNode;
  /** Renders a filter box that matches across every cell of a row. */
  searchable?: boolean;
  searchPlaceholder?: string;
  /** Rows per page when the browser paginates. 0 shows everything at once. */
  pageSize?: number;
  /** Set when the server paginates: `dataSource` is then already the current page. */
  serverPagination?: ServerPagination;
  onSort?: (field: string, order: SortOrder) => void;
  emptyText?: React.ReactNode;
  emptyIcon?: React.ComponentType<{className?: string}>;
  /** Why the listing is empty when it is empty because the load failed. */
  error?: string;
  /** Reloads the listing from the error state. */
  onRetry?: () => void;
  onRowClick?: (record: T) => void;
  expandable?: {
    rowExpandable?: (record: T) => boolean;
    expandedRowRender: (record: T) => React.ReactNode;
  };
  /** Tighter row padding for tables embedded in a card or a row. */
  dense?: boolean;
  className?: string;
  tableClassName?: string;
}

/**
 * The list-page workhorse. Every resource screen renders the same thing: an
 * optional toolbar, a bordered table, and pagination that only appears once the
 * data actually overflows a page.
 */
export function DataTable<T>({
  columns,
  dataSource,
  rowKey = "name",
  loading = false,
  title,
  description,
  toolbar,
  searchable = false,
  searchPlaceholder,
  pageSize = 20,
  serverPagination,
  onSort,
  emptyText,
  emptyIcon,
  error,
  onRetry,
  onRowClick,
  expandable,
  dense = false,
  className,
  tableClassName,
}: DataTableProps<T>) {
  const [sorting, setSorting] = React.useState<SortingState>([]);
  const [globalFilter, setGlobalFilter] = React.useState("");
  const [expanded, setExpanded] = React.useState<Record<string, boolean>>({});

  const data = React.useMemo(() => dataSource ?? [], [dataSource]);

  // A column that says the server sorts is never sorted here: the click is
  // reported through onSort and the backend sends the next page back.
  const serverSorted = React.useMemo(() => columns.some(column => column.sorter === true), [columns]);

  const columnDefs = React.useMemo<ColumnDef<T>[]>(
    () =>
      columns.map((column, columnIndex) => {
        const id = column.key ?? column.dataIndex ?? `col-${columnIndex}`;
        const shared = {
          id: String(id),
          header: () => column.title,
          enableSorting: Boolean(column.sorter) && column.dataIndex !== undefined,
          sortingFn:
            typeof column.sorter === "function"
              ? (rowA: any, rowB: any) => (column.sorter as (a: T, b: T) => number)(rowA.original, rowB.original)
              : undefined,
          meta: column,
        };

        if (column.dataIndex === undefined) {
          return {
            ...shared,
            cell: ({row}: any) => (column.render ? column.render(undefined, row.original, row.index) : null),
          } as ColumnDef<T>;
        }

        return {
          ...shared,
          accessorFn: (row: T) => readPath(row, column.dataIndex),
          cell: ({row, getValue}: any) =>
            column.render ? column.render(getValue(), row.original, row.index) : getValue(),
        } as ColumnDef<T>;
      }),
    [columns],
  );

  const browserPaginates = pageSize > 0 && !serverPagination;

  const table = useReactTable({
    data,
    columns: columnDefs,
    state: {sorting, globalFilter},
    onSortingChange: updater => {
      const next = typeof updater === "function" ? updater(sorting) : updater;
      setSorting(next);
      if (serverSorted && onSort) {
        const entry = next[0];
        const column = entry ? columns.find(item => item.key === entry.id) : undefined;
        onSort(column?.dataIndex ?? entry?.id ?? "", entry ? (entry.desc ? "descend" : "ascend") : undefined);
      }
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: browserPaginates ? getPaginationRowModel() : undefined,
    manualSorting: serverSorted,
    getRowId: (record, index) => resolveRowKey(rowKey, record, index),
    // Pages that poll hand over a fresh array every few seconds, which would
    // otherwise throw the reader back to page 1 mid-read.
    autoResetPageIndex: false,
    // The default filter only sees accessor values, which would silently skip
    // display columns. Matching the whole record keeps a search for a nested
    // field working the way a reader expects.
    globalFilterFn: (row, _columnId, value) =>
      JSON.stringify(row.original ?? {})
        .toLowerCase()
        .includes(String(value).toLowerCase()),
    initialState: browserPaginates ? {pagination: {pageSize}} : undefined,
  });

  const rows = table.getRowModel().rows;
  const showHeader = Boolean(title || description || toolbar || searchable);
  const totalRows = table.getFilteredRowModel().rows.length;

  const currentPage = serverPagination ? serverPagination.page : table.getState().pagination.pageIndex + 1;
  const currentPageSize = serverPagination ? serverPagination.pageSize : table.getState().pagination.pageSize;
  const total = serverPagination ? serverPagination.total : totalRows;
  const pageCount = serverPagination
    ? Math.max(1, Math.ceil(serverPagination.total / Math.max(1, serverPagination.pageSize)))
    : table.getPageCount();
  const showPagination = serverPagination ? total > currentPageSize || currentPage > 1 : browserPaginates && total > pageSize;

  const pageIndex = table.getState().pagination.pageIndex;

  React.useEffect(() => {
    if (browserPaginates) {
      table.setPageIndex(0);
    }
  }, [browserPaginates, globalFilter, table]);

  // The rows behind the current page can go away while it is open.
  React.useEffect(() => {
    if (browserPaginates && pageIndex > 0 && pageIndex >= pageCount) {
      table.setPageIndex(Math.max(0, pageCount - 1));
    }
  }, [browserPaginates, pageCount, pageIndex, table]);

  const goTo = (nextPage: number) => {
    if (serverPagination) {
      serverPagination.onChange(nextPage, serverPagination.pageSize);
      return;
    }
    table.setPageIndex(nextPage - 1);
  };

  return (
    <div
      data-slot="data-table"
      data-loading={loading ? "true" : "false"}
      className={cn("bg-card flex flex-col overflow-hidden rounded-xl border shadow-sm", className)}
    >
      {showHeader && (
        <div
          data-slot="data-table-header"
          className="flex flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
        >
          <div className="min-w-0">
            {title ? <h2 className="truncate text-sm font-semibold">{title}</h2> : null}
            {description ? <p className="text-muted-foreground mt-0.5 truncate text-xs">{description}</p> : null}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {searchable && (
              <div className="relative">
                <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2" />
                <Input
                  value={globalFilter}
                  onChange={event => setGlobalFilter(event.target.value)}
                  placeholder={searchPlaceholder ?? i18next.t("general:Search")}
                  className="h-8 w-44 pl-8 text-xs lg:w-56"
                />
              </div>
            )}
            {toolbar}
          </div>
        </div>
      )}

      <Table className={tableClassName} containerClassName="scrollbar-thin">
        <TableHeader className="bg-muted/40">
          {table.getHeaderGroups().map(headerGroup => (
            <TableRow key={headerGroup.id} className="hover:bg-transparent">
              {expandable ? <TableHead className="w-8" /> : null}
              {headerGroup.headers.map(header => {
                const meta = (header.column.columnDef.meta ?? {}) as Column<T>;
                const canSort = header.column.getCanSort();
                return (
                  <TableHead
                    key={header.id}
                    style={meta.width ? {width: meta.width, minWidth: meta.width} : undefined}
                    className={cn(
                      meta.align === "right" && "text-right",
                      meta.align === "center" && "text-center",
                      meta.headerClassName,
                    )}
                  >
                    {canSort ? (
                      <button
                        type="button"
                        onClick={header.column.getToggleSortingHandler()}
                        className="hover:text-foreground inline-flex items-center gap-1 transition-colors"
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        <SortIcon state={header.column.getIsSorted()} />
                      </button>
                    ) : (
                      flexRender(header.column.columnDef.header, header.getContext())
                    )}
                  </TableHead>
                );
              })}
            </TableRow>
          ))}
        </TableHeader>

        <TableBody>
          {loading && rows.length === 0 ? (
            Array.from({length: 5}).map((_, rowIndex) => (
              <TableRow key={`skeleton-${rowIndex}`} className="hover:bg-transparent">
                {expandable ? <TableCell /> : null}
                {columns.map((column, columnIndex) => (
                  <TableCell key={column.key ?? columnIndex}>
                    <Skeleton className="h-4 w-full max-w-[160px]" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : rows.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell colSpan={columns.length + (expandable ? 1 : 0)} className="p-0">
                {error ? (
                  <ErrorState error={error} onRetry={onRetry} />
                ) : (
                  <EmptyState icon={emptyIcon} title={emptyText ?? i18next.t("general:No data")} />
                )}
              </TableCell>
            </TableRow>
          ) : (
            rows.map(row => {
              const isExpanded = Boolean(expanded[row.id]);
              const canExpand = expandable
                ? expandable.rowExpandable
                  ? expandable.rowExpandable(row.original)
                  : true
                : false;
              return (
                <React.Fragment key={row.id}>
                  <TableRow
                    data-row-key={row.id}
                    data-state={isExpanded ? "selected" : undefined}
                    onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                    className={cn(onRowClick && "cursor-pointer")}
                  >
                    {expandable ? (
                      <TableCell className="w-8 pr-0">
                        {canExpand ? (
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            aria-label={isExpanded ? "Collapse row" : "Expand row"}
                            onClick={event => {
                              event.stopPropagation();
                              setExpanded(prev => ({...prev, [row.id]: !prev[row.id]}));
                            }}
                          >
                            <ChevronRight className={cn("size-3.5 transition-transform", isExpanded && "rotate-90")} />
                          </Button>
                        ) : null}
                      </TableCell>
                    ) : null}
                    {row.getVisibleCells().map(cell => {
                      const meta = (cell.column.columnDef.meta ?? {}) as Column<T>;
                      return (
                        <TableCell
                          key={cell.id}
                          style={meta.width ? {width: meta.width, minWidth: meta.width} : undefined}
                          className={cn(
                            dense && "py-1.5",
                            meta.align === "right" && "text-right",
                            meta.align === "center" && "text-center",
                            meta.ellipsis && "max-w-0 truncate",
                            meta.className,
                          )}
                        >
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </TableCell>
                      );
                    })}
                  </TableRow>
                  {canExpand && isExpanded && expandable ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={columns.length + 1} className="bg-muted/30 p-0">
                        {expandable.expandedRowRender(row.original)}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </React.Fragment>
              );
            })
          )}
        </TableBody>
      </Table>

      {showPagination && (
        <div className="flex items-center justify-between border-t px-4 py-2.5">
          <span className="text-muted-foreground text-xs">
            {(currentPage - 1) * currentPageSize + 1}
            {"–"}
            {Math.min(currentPage * currentPageSize, total)} of {total}
          </span>
          <div className="flex items-center gap-1">
            <Button
              variant="outline"
              size="icon-sm"
              onClick={() => goTo(currentPage - 1)}
              disabled={currentPage <= 1}
              aria-label="Previous page"
            >
              <ChevronLeft className="size-4" />
            </Button>
            <span className="text-muted-foreground px-2 text-xs tabular-nums">
              {currentPage} / {pageCount}
            </span>
            <Button
              variant="outline"
              size="icon-sm"
              onClick={() => goTo(currentPage + 1)}
              disabled={currentPage >= pageCount}
              aria-label="Next page"
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
