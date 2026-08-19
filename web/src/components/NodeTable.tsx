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
import {CircleCheck, CircleMinus, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {RowActions} from "@/components/rules/RowActions";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Tooltip} from "@/components/ui/tooltip";
import type {Account, Node, SiteNode} from "@/types";

function StatusBadge({status}: {status: string}) {
  if (status === "") {
    return null;
  }
  if (status === "In Progress") {
    return (
      <Badge variant="processing">
        <RefreshCw className="h-3 w-3 animate-spin" />
        {status}
      </Badge>
    );
  }
  if (status === "Running") {
    return (
      <Badge variant="success">
        <CircleCheck className="h-3 w-3" />
        {status}
      </Badge>
    );
  }
  if (status === "Stopped") {
    return (
      <Badge variant="error">
        <CircleMinus className="h-3 w-3" />
        {status}
      </Badge>
    );
  }
  return <>{status}</>;
}

/** The per-site deployment table on the site edit page. */
export function NodeTable({
  title,
  table,
  siteName,
  account,
  nodes,
  onUpdateTable,
}: {
  title: React.ReactNode;
  table: SiteNode[] | undefined;
  siteName: string;
  account: Account;
  nodes: Node[];
  onUpdateTable: (table: SiteNode[]) => void;
}) {
  const rows = React.useMemo(() => table ?? [], [table]);

  const updateField = (index: number, key: keyof SiteNode, value: string) => {
    onUpdateTable(rows.map((row, i) => (i === index ? {...row, [key]: value} : row)));
  };

  const addRow = () => {
    const row: SiteNode = {name: `New Node - ${rows.length}`, version: "", diff: "", status: "", message: ""};
    if (rows.length === 0 && account.hostname) {
      row.name = account.hostname;
    }
    onUpdateTable(Setting.addRow(rows, row));
  };

  // A node can only be deployed once per site, so the ones already listed are
  // dropped from the other rows' choices.
  const selectedNames = rows.map(row => row.name);
  const availableNodes = (currentName: string) =>
    nodes.filter(node => node.name === currentName || !selectedNames.includes(node.name));

  const columns: Column<SiteNode>[] = [
    {
      title: "Name",
      key: "name",
      dataIndex: "name",
      width: "200px",
      render: (text: string, _record, index) => (
        <Select value={text || undefined} onValueChange={value => updateField(index, "name", value)}>
          <SelectTrigger>
            <SelectValue placeholder={i18next.t("node:Node")} />
          </SelectTrigger>
          <SelectContent>
            {availableNodes(text).map(node => (
              <SelectItem key={node.name} value={node.name}>
                {node.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ),
    },
    {
      title: "Version",
      key: "version",
      dataIndex: "version",
      width: "160px",
      render: (text: string) => {
        const versionInfo = Setting.getVersionInfo(text, siteName);
        if (versionInfo === null) {
          return null;
        }
        return (
          <a target="_blank" rel="noreferrer" href={versionInfo.link} className="text-primary hover:underline">
            {versionInfo.text}
          </a>
        );
      },
    },
    {
      title: "Diff",
      key: "diff",
      dataIndex: "diff",
      render: (text: string, record) => {
        if (record.status === "") {
          return null;
        }
        return (
          <Tooltip title={<pre className="max-h-96 overflow-auto whitespace-pre-wrap text-xs">{text}</pre>}>
            <span className="cursor-help">{Setting.getShortText(text)}</span>
          </Tooltip>
        );
      },
    },
    {
      title: "Pid",
      key: "pid",
      dataIndex: "pid",
      width: "110px",
      render: (text: number) => (text === 0 || text === undefined ? null : text),
    },
    {
      title: "Status",
      key: "status",
      dataIndex: "status",
      width: "150px",
      render: (text: string) => <StatusBadge status={text} />,
    },
    {
      title: "Message",
      key: "message",
      dataIndex: "message",
    },
    {
      title: "Provider",
      key: "provider",
      dataIndex: "provider",
      width: "220px",
      render: (text: string, _record, index) => (
        <Input value={text ?? ""} onChange={event => updateField(index, "provider", event.target.value)} />
      ),
    },
    {
      title: "Action",
      key: "action",
      width: "130px",
      render: (_text, _record, index) => (
        <RowActions
          index={index}
          length={rows.length}
          onUp={() => onUpdateTable(Setting.swapRow(rows, index - 1, index))}
          onDown={() => onUpdateTable(Setting.swapRow(rows, index, index + 1))}
          onDelete={() => onUpdateTable(Setting.deleteRow(rows, index))}
        />
      ),
    },
  ];

  return (
    <DataTable
      columns={columns}
      data={rows}
      rowKey={(_record, index) => String(index)}
      pageSize={0}
      title={title}
      toolbar={
        <Button size="sm" onClick={addRow}>
          Add
        </Button>
      }
    />
  );
}
