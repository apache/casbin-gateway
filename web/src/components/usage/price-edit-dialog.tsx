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
import {Download} from "lucide-react";
import i18next from "i18next";

import {FormDialog} from "@/components/shared/form-dialog";
import {ModelsDevPicker} from "@/components/usage/models-dev-picker";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import type {LlmPriceEntry, LlmPriceView, ModelsDevModel} from "@/types";

/** The rates one price is made of, in the order the table shows them. */
type RateField = "input" | "output" | "cacheWrite" | "cacheRead" | "cacheWrite1h";

const RATE_FIELDS: {field: RateField; label: string; hint?: string}[] = [
  {field: "input", label: "usage:Input"},
  {field: "output", label: "llm:Output"},
  {field: "cacheWrite", label: "llm:Cache write"},
  {field: "cacheRead", label: "llm:Cache read"},
  {field: "cacheWrite1h", label: "usage:Cache write 1h", hint: "usage:Cache write 1h hint"},
];

/** The form's own state. Rates are held as text so a half-typed "0." survives
 *  the keystroke that would otherwise round it away. */
interface PriceDraft {
  model: string;
  displayName: string;
  rates: Record<RateField, string>;
}

const emptyRates: Record<RateField, string> = {
  input: "0",
  output: "0",
  cacheWrite: "0",
  cacheRead: "0",
  cacheWrite1h: "0",
};

function draftOf(price: LlmPriceView | null): PriceDraft {
  if (price === null) {
    return {model: "", displayName: "", rates: {...emptyRates}};
  }
  return {
    model: price.model,
    displayName: price.displayName,
    rates: {
      input: String(price.input),
      output: String(price.output),
      cacheWrite: String(price.cacheWrite),
      cacheRead: String(price.cacheRead),
      cacheWrite1h: String(price.cacheWrite1h),
    },
  };
}

function rateOf(text: string) {
  const parsed = Number.parseFloat(text);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

/**
 * Edits what one model costs. A price saved here is a manual one from then on,
 * which is what keeps it from being overwritten by the next models.dev sync.
 */
export function PriceEditDialog({
  open,
  onOpenChange,
  price,
  onSave,
  saving,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Null adds a price rather than editing one. */
  price: LlmPriceView | null;
  onSave: (entry: Partial<LlmPriceEntry>) => void;
  saving?: boolean;
}) {
  const [draft, setDraft] = React.useState<PriceDraft>(() => draftOf(price));
  const [picking, setPicking] = React.useState(false);

  // The dialog is kept mounted between rows, so the draft is reset whenever it
  // is opened on a different price rather than on first render only.
  React.useEffect(() => {
    if (open) {
      setDraft(draftOf(price));
    }
  }, [open, price]);

  const adding = price === null;

  const setRate = (field: RateField, value: string) =>
    setDraft(current => ({...current, rates: {...current.rates, [field]: value}}));

  const fillFrom = (model: ModelsDevModel) =>
    setDraft(current => ({
      model: current.model.trim() === "" ? model.model : current.model,
      displayName: model.displayName,
      rates: {
        ...current.rates,
        input: String(model.input),
        output: String(model.output),
        cacheWrite: String(model.cacheWrite),
        cacheRead: String(model.cacheRead),
      },
    }));

  const submit = () =>
    onSave({
      model: draft.model,
      displayName: draft.displayName,
      input: rateOf(draft.rates.input),
      output: rateOf(draft.rates.output),
      cacheWrite: rateOf(draft.rates.cacheWrite),
      cacheRead: rateOf(draft.rates.cacheRead),
      cacheWrite1h: rateOf(draft.rates.cacheWrite1h),
    });

  return (
    <>
      <FormDialog
        open={open}
        onOpenChange={onOpenChange}
        title={adding ? i18next.t("usage:Add a price") : i18next.t("usage:Edit price")}
        description={i18next.t("usage:Price dialog description")}
        onSubmit={submit}
        submitting={saving}
        submitDisabled={draft.model.trim() === ""}
      >
        <div className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="price-model">{i18next.t("agent:Model")}</Label>
              <Input
                id="price-model"
                // The key is what a request is matched against, so an existing
                // one is not editable: changing it would leave the old row in
                // place and quietly add a second.
                disabled={!adding}
                className="font-mono"
                placeholder="claude-opus-5"
                value={draft.model}
                onChange={event => setDraft(current => ({...current, model: event.target.value}))}
              />
              <span className="text-muted-foreground text-xs">{i18next.t("usage:Model key hint")}</span>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="price-name">{i18next.t("general:Display name")}</Label>
              <Input
                id="price-name"
                placeholder="Claude Opus 5"
                value={draft.displayName}
                onChange={event => setDraft(current => ({...current, displayName: event.target.value}))}
              />
              <Button type="button" variant="outline" size="sm" onClick={() => setPicking(true)}>
                <Download />
                {i18next.t("usage:Import from models.dev")}
              </Button>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {RATE_FIELDS.map(({field, label, hint}) => (
              <div key={field} className="flex flex-col gap-2">
                <Label htmlFor={`price-${field}`}>{i18next.t(label)}</Label>
                <Input
                  id={`price-${field}`}
                  type="number"
                  min={0}
                  step="any"
                  className="tabular-nums"
                  value={draft.rates[field]}
                  onChange={event => setRate(field, event.target.value)}
                />
                {hint ? <span className="text-muted-foreground text-xs">{i18next.t(hint)}</span> : null}
              </div>
            ))}
          </div>

          <span className="text-muted-foreground text-xs">{i18next.t("usage:Rates are per million tokens")}</span>
        </div>
      </FormDialog>

      <ModelsDevPicker open={picking} onOpenChange={setPicking} onPick={fillFrom} />
    </>
  );
}
