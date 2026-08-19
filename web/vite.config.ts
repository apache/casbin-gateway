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

import path from "path";
import react from "@vitejs/plugin-react";
import {defineConfig} from "vite";

// The dev server proxies the API to the Go backend instead of calling it at an
// absolute URL, so the browser sees one origin and the beego session cookie is
// stored and replayed without any CORS or SameSite special cases. In production
// the same is true for free: the backend serves this bundle itself.
const backend = process.env.VITE_BACKEND_URL || "http://localhost:17000";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 16002,
    proxy: {
      "/api": {target: backend, changeOrigin: false},
      "/v1": {target: backend, changeOrigin: false},
    },
  },
  build: {
    // The Go side looks for the compiled UI in web/build, so the output goes
    // there rather than Vite's default "dist".
    outDir: "build",
    sourcemap: false,
  },
});
