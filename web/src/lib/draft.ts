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

/**
 * A half-filled form kept in the browser so closing the dialog — or the tab —
 * does not throw the typing away. Drafts live only here: nothing is sent to the
 * server until the form is submitted.
 */
export interface Draft<T> {
  value: T;
  savedAt: number;
}

const prefix = "casbin-gateway:draft:";

export function readDraft<T>(key: string): Draft<T> | null {
  try {
    const raw = localStorage.getItem(prefix + key);
    if (!raw) {
      return null;
    }
    const draft = JSON.parse(raw) as Draft<T>;
    return draft && draft.value !== undefined ? draft : null;
  } catch {
    return null;
  }
}

export function writeDraft<T>(key: string, value: T): Draft<T> {
  const draft = {value: value, savedAt: Date.now()};
  try {
    localStorage.setItem(prefix + key, JSON.stringify(draft));
  } catch {
    // A full or blocked storage costs the draft, not the form.
  }
  return draft;
}

export function clearDraft(key: string) {
  try {
    localStorage.removeItem(prefix + key);
  } catch {
    // Nothing to do: the draft is already unreachable.
  }
}
