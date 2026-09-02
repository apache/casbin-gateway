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

/** US dollars, with enough digits left for a single cheap request to show up. */
export function formatCost(cost: number) {
  if (cost <= 0) {
    return "$0";
  }
  if (cost < 0.01) {
    return `$${cost.toFixed(4)}`;
  }
  if (cost < 1) {
    return `$${cost.toFixed(3)}`;
  }
  return `$${cost.toFixed(2)}`;
}

export function formatTokens(tokens: number) {
  if (tokens < 1000) {
    return String(tokens);
  }
  if (tokens < 1000000) {
    return `${(tokens / 1000).toFixed(1)}k`;
  }
  return `${(tokens / 1000000).toFixed(2)}M`;
}

/**
 * What one window spent, whichever source counted it: the records Gateway
 * relayed, or the transcripts the agents wrote themselves. The dashboard draws
 * one set of cards from either, so both are normalised to this first.
 */
export interface UsageTotals {
  requests: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
  /** Requests left out of the cost because no list price matched their model. */
  unpriced: number;
  /** Only the relayed source knows this; the transcripts record no outcome. */
  failed?: number;
}

/** One row of a breakdown: what a single model, provider or agent spent. */
export interface UsageRow {
  name: string;
  requests: number;
  tokens: number;
  cost: number;
  failed?: number;
  /** What the row was last seen doing, where the source knows it. */
  detail?: string;
}

/** One point of the trend, with the tokens split the way the chart stacks them. */
export interface UsagePoint {
  bucket: string;
  requests: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  cost: number;
}

export const emptyUsageTotals: UsageTotals = {
  requests: 0,
  promptTokens: 0,
  completionTokens: 0,
  cacheReadTokens: 0,
  cacheWriteTokens: 0,
  totalTokens: 0,
  cost: 0,
  unpriced: 0,
};

/**
 * Share of the input tokens that were served out of the prompt cache. Written
 * over the input alone, because the output tokens were never cacheable and
 * counting them would quietly hold every rate down.
 */
export function cacheHitRate(tokens: {promptTokens: number; cacheReadTokens: number; cacheWriteTokens: number}) {
  const input = tokens.promptTokens + tokens.cacheReadTokens + tokens.cacheWriteTokens;
  return input === 0 ? 0 : (tokens.cacheReadTokens / input) * 100;
}

/**
 * The label a bucket key gets on the axis. A key is either "2026-09-02" or
 * "2026-09-02T14", cut out of a timestamp by the server, so it is shortened
 * rather than parsed: reading it as a Date would move it into the browser's own
 * zone and label the point with an hour it was never counted in.
 */
export function bucketLabel(bucket: string) {
  const hour = bucket.indexOf("T");
  return hour === -1 ? bucket.slice(5) : `${bucket.slice(11)}:00`;
}

/** A Date as the calendar day it falls on locally, spelled the way the day
 *  buckets are: the agents write their transcripts in this machine's own zone,
 *  and toISOString would name a different day for half of it. */
function localDay(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}

/**
 * Fills in the days a source skipped because nothing was spent on them, over
 * the whole window that was asked for rather than only between the days that
 * have data: a quiet week is a flat stretch in the chart, not a chart that is
 * six days shorter than its own label.
 *
 * `days` is the window in days, 0 for everything the source holds — which has
 * no start of its own, so those are filled between the days present. Only day
 * buckets are filled; the relayed source has its gaps filled by the server,
 * which is the side that knows where its window begins.
 */
export function fillDays(points: UsagePoint[], days: number): UsagePoint[] {
  if (points.length === 0) {
    return points;
  }

  const step = 24 * 60 * 60 * 1000;
  // Parsed and stepped through in UTC: the keys are calendar days, and walking
  // them in the local zone would drop or repeat one wherever the clock moves.
  const at = (key: string) => Date.parse(`${key}T00:00:00Z`);
  const keyOf = (stamp: number) => new Date(stamp).toISOString().slice(0, 10);

  const today = localDay(new Date());
  const oldest = points[0].bucket;
  const newest = points[points.length - 1].bucket;
  const first = days > 0 ? localDay(new Date(at(today) - (days - 1) * step)) : oldest;

  const start = at(first < oldest ? first : oldest);
  const end = at(newest > today ? newest : today);
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) {
    return points;
  }

  const known = new Map(points.map(point => [point.bucket, point]));
  const filled: UsagePoint[] = [];
  for (let stamp = start; stamp <= end && filled.length < 1500; stamp += step) {
    const key = keyOf(stamp);
    filled.push(
      known.get(key) ?? {
        bucket: key,
        requests: 0,
        promptTokens: 0,
        completionTokens: 0,
        cacheReadTokens: 0,
        cacheWriteTokens: 0,
        cost: 0,
      },
    );
  }
  return filled.length === 0 ? points : filled;
}
