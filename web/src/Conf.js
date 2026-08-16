// Copyright 2023 The casbin Authors. All Rights Reserved.
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

// These default to the public Casdoor demo so a plain `yarn start` works out of
// the box. The docker-compose quickstart overrides them at build time via
// REACT_APP_* build args (see Dockerfile) so the browser talks to the bundled,
// self-hosted Casdoor instead.
export const AuthConfig = {
  serverUrl: process.env.REACT_APP_CASDOOR_SERVER_URL || "https://door.casdoor.com",
  clientId: process.env.REACT_APP_CLIENT_ID || "af6b5aa958822fb9dc33",
  appName: process.env.REACT_APP_APP_NAME || "app-casibase",
  organizationName: process.env.REACT_APP_ORG_NAME || "casbin",
  redirectPath: "/callback",
};

export const IsDemoMode = false;

export const ForceLanguage = "";
export const DefaultLanguage = "en";
