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

import {ArrowDown, ArrowUp, Trash2} from "lucide-react";

import {Button} from "@/components/ui/button";
import {Tooltip} from "@/components/ui/tooltip";

/** The move-up / move-down / delete trio every editable sub-table carries. */
export function RowActions({
  index,
  length,
  onUp,
  onDown,
  onDelete,
}: {
  index: number;
  length: number;
  onUp: () => void;
  onDown: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center gap-1">
      <Tooltip title="Up">
        <Button variant="outline" size="icon-sm" disabled={index === 0} onClick={onUp}>
          <ArrowUp />
        </Button>
      </Tooltip>
      <Tooltip title="Down">
        <Button variant="outline" size="icon-sm" disabled={index === length - 1} onClick={onDown}>
          <ArrowDown />
        </Button>
      </Tooltip>
      <Tooltip title="Delete">
        <Button variant="outline" size="icon-sm" onClick={onDelete}>
          <Trash2 />
        </Button>
      </Tooltip>
    </div>
  );
}
