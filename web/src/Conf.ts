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

// Casdoor is optional. These values stay empty unless the backend reports a
// configured "casdoorEndpoint" in app.conf, in which case setAuthConfig() fills
// them in at startup and the app signs in through Casdoor instead of the
// built-in username/password form.
export const AuthConfig = {
  serverUrl: "",
  clientId: "",
  appName: "",
  organizationName: "",
  redirectPath: "/callback",
};

export function setAuthConfig(config: Partial<typeof AuthConfig> | null | undefined) {
  if (config === null || config === undefined) {
    return;
  }

  (Object.keys(AuthConfig) as (keyof typeof AuthConfig)[]).forEach(key => {
    const value = config[key];
    if (value !== undefined && value !== null) {
      AuthConfig[key] = value;
    }
  });
}

export const IsDemoMode = false;

export const ForceLanguage = "";
export const DefaultLanguage = "en";
