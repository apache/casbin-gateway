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
import {useNavigate, useParams} from "react-router-dom";
import i18next from "i18next";

import * as RuleBackend from "@/backend/RuleBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {CompoundRule} from "@/components/rules/CompoundRule";
import {IpRateRuleTable} from "@/components/rules/IpRateRuleTable";
import {IpRuleTable} from "@/components/rules/IpRuleTable";
import {UaRuleTable} from "@/components/rules/UaRuleTable";
import {WafRuleTable} from "@/components/rules/WafRuleTable";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {NumberInput} from "@/components/ui/number-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {PageSpinner} from "@/components/ui/spinner";
import {Switch} from "@/components/ui/switch";
import type {Rule, RuleExpression} from "@/types";

export default function RuleEditPage() {
  const {owner = "", ruleName = ""} = useParams();
  const navigate = useNavigate();
  const [rule, setRule] = React.useState<Rule | null>(null);

  const getRule = React.useCallback(() => {
    RuleBackend.getRule(owner, ruleName).then(res => {
      if (res.status === "ok") {
        setRule(res.data);
      } else {
        Setting.showMessage("error", `Failed to get rule: ${res.msg}`);
      }
    });
  }, [owner, ruleName]);

  React.useEffect(() => {
    getRule();
  }, [getRule]);

  const updateField = <K extends keyof Rule>(key: K, value: Rule[K]) => {
    setRule(current => {
      if (current === null) {
        return current;
      }
      const next = {...current, [key]: value};
      // Expressions are shaped by the rule type, so switching type discards
      // the ones that belonged to the old type.
      if (key === "type") {
        next.expressions = [];
      }
      return next;
    });
  };

  const save = () => {
    if (rule === null) {
      return;
    }

    RuleBackend.updateRule(owner, ruleName, Setting.deepCopy(rule)).then(res => {
      if (res.status !== "error") {
        Setting.showMessage("success", "Rule updated successfully");
        navigate(`/rules/${rule.owner}/${rule.name}`);
      } else {
        Setting.showMessage("error", `Rule failed to update: ${res.msg}`);
        getRule();
      }
    });
  };

  if (rule === null) {
    return <PageSpinner />;
  }

  const setExpressions = (expressions: RuleExpression[]) => updateField("expressions", expressions);

  const renderExpressions = () => {
    switch (rule.type) {
    case "WAF":
      return <WafRuleTable title="Seclang" table={rule.expressions} onUpdateTable={setExpressions} />;
    case "IP":
      return <IpRuleTable title="IPs" table={rule.expressions} onUpdateTable={setExpressions} />;
    case "User-Agent":
      return (
        <UaRuleTable title="User-Agents" table={rule.expressions} onUpdateTable={setExpressions} />
      );
    case "URL Path":
      return (
        <UaRuleTable
          kind="urlPath"
          title="URL Paths"
          table={rule.expressions}
          onUpdateTable={setExpressions}
        />
      );
    case "IP Rate Limiting":
      return (
        <IpRateRuleTable
          title={i18next.t("rule:IP Rate Limiting")}
          table={rule.expressions}
          onUpdateTable={setExpressions}
        />
      );
    case "Compound":
      return (
        <CompoundRule
          title={i18next.t("rule:Compound")}
          table={rule.expressions}
          owner={owner}
          ruleName={rule.name}
          onUpdateTable={setExpressions}
        />
      );
    default:
      return null;
    }
  };

  const types = [
    {value: "WAF", label: "WAF"},
    {value: "IP", label: "IP"},
    {value: "User-Agent", label: "User-Agent"},
    {value: "URL Path", label: "URL Path"},
    {value: "IP Rate Limiting", label: i18next.t("rule:IP Rate Limiting")},
    {value: "Compound", label: i18next.t("rule:Compound")},
  ];

  const actions = [
    {value: "Allow", label: i18next.t("rule:Allow")},
    {value: "Block", label: i18next.t("rule:Block")},
    {value: "CAPTCHA", label: i18next.t("rule:Captcha")},
  ];

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("rule:Edit Rule")}>
        <Button variant="outline" onClick={() => navigate("/rules")}>
          {i18next.t("general:Cancel")}
        </Button>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={rule.name} onChange={event => updateField("name", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("rule:Type")}>
            <Select value={rule.type} onValueChange={value => updateField("type", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {types.map(type => (
                  <SelectItem key={type.value} value={type.value}>
                    {type.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("rule:Expressions")}>
            {/* Keying on the type remounts the table when the type changes, so
                it seeds that type's default expressions. User-Agent and URL
                Path share one component and would otherwise stay put. */}
            <React.Fragment key={rule.type}>{renderExpressions()}</React.Fragment>
          </FormRow>
          {rule.type !== "WAF" && (
            <FormRow label={i18next.t("general:Action")}>
              <Select value={rule.action} onValueChange={value => updateField("action", value)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {actions.map(action => (
                    <SelectItem key={action.value} value={action.value}>
                      {action.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FormRow>
          )}
          {rule.type !== "WAF" && (rule.action === "Allow" || rule.action === "Block") && (
            <FormRow label={i18next.t("rule:Status code")}>
              <NumberInput
                min={100}
                max={599}
                className="max-w-xs"
                value={rule.statusCode}
                onChange={value => updateField("statusCode", value)}
              />
            </FormRow>
          )}
          <FormRow label={i18next.t("rule:Reason")}>
            <Input value={rule.reason ?? ""} onChange={event => updateField("reason", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("rule:Verbose mode")}>
            <Switch
              checked={rule.isVerbose}
              onCheckedChange={checked => updateField("isVerbose", checked)}
            />
          </FormRow>
        </CardContent>
      </Card>

      <div className="mt-4">
        <Button size="lg" onClick={save}>
          {i18next.t("general:Save")}
        </Button>
      </div>
    </div>
  );
}
