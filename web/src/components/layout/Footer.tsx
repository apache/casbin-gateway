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

import * as Setting from "@/Setting";

export function Footer() {
  return (
    <footer className="mt-auto border-t py-4">
      <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
        Powered by
        <a
          target="_blank"
          rel="noreferrer"
          href="https://github.com/apache/casbin-gateway"
          className="inline-flex items-center"
        >
          <img
            className="h-5 w-auto"
            alt="Casbin"
            src={`${Setting.StaticBaseUrl}/img/casbin_logo_1024x256.png`}
          />
        </a>
      </div>
    </footer>
  );
}
