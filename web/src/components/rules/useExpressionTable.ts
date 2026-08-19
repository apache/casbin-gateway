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

import * as Setting from "@/Setting";
import type {RuleExpression} from "@/types";

export interface ExpressionTableProps {
  title: React.ReactNode;
  table: RuleExpression[];
  onUpdateTable: (table: RuleExpression[]) => void;
}

/**
 * The row bookkeeping every rule expression table shares: edit one field, move
 * a row, delete a row, and reset to the type's default rules.
 *
 * `defaultRules` is also applied on first render when the rule has no
 * expressions yet, which is how the antd tables seeded a freshly created rule.
 */
export function useExpressionTable(
  table: RuleExpression[],
  onUpdateTable: (table: RuleExpression[]) => void,
  defaultRules: RuleExpression[],
) {
  const rows = React.useMemo(() => table ?? [], [table]);
  const seeded = React.useRef(false);

  React.useEffect(() => {
    if (!seeded.current && rows.length === 0) {
      seeded.current = true;
      onUpdateTable(defaultRules);
    }
    // Seeding must happen once, on the first render that finds the rule empty.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const updateField = (index: number, key: keyof RuleExpression, value: string) => {
    const next = rows.map((row, i) => (i === index ? {...row, [key]: value} : row));
    onUpdateTable(next);
  };

  const addRow = (row: RuleExpression) => {
    onUpdateTable(Setting.addRow(rows, row));
  };

  const deleteRow = (index: number) => {
    onUpdateTable(Setting.deleteRow(rows, index));
  };

  const upRow = (index: number) => {
    onUpdateTable(Setting.swapRow(rows, index - 1, index));
  };

  const downRow = (index: number) => {
    onUpdateTable(Setting.swapRow(rows, index, index + 1));
  };

  const restore = () => {
    onUpdateTable(defaultRules);
  };

  return {rows, updateField, addRow, deleteRow, upRow, downRow, restore};
}
