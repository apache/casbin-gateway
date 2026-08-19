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

import {itemId, query, request} from "@/backend/request";
import type {Channel, ChannelTestResult} from "@/types";

export function getChannels(
  owner: string,
  page: string | number = "",
  pageSize: string | number = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Channel[], number>(
    `/api/get-channels${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getChannel(owner: string, name: string) {
  return request<Channel>(`/api/get-channel${query({id: itemId(owner, name)})}`);
}

export function addChannel(channel: Channel) {
  return request("/api/add-channel", "POST", channel);
}

export function updateChannel(owner: string, name: string, channel: Channel) {
  return request(`/api/update-channel${query({id: itemId(owner, name)})}`, "POST", channel);
}

export function deleteChannel(channel: Channel) {
  return request("/api/delete-channel", "POST", channel);
}

export function testChannel(owner: string, name: string) {
  return request<ChannelTestResult>("/api/test-channel", "POST", {owner: owner, name: name});
}
