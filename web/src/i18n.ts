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

import i18n from "i18next";
import {initReactI18next} from "react-i18next";

import * as Conf from "@/Conf";
import en from "@/locales/en/data.json";
import zh from "@/locales/zh/data.json";

const resources = {
  en: en,
  zh: zh,
};

function initLanguage() {
  let language = localStorage.getItem("language");
  if (language === undefined || language === null) {
    if (Conf.ForceLanguage !== "") {
      language = Conf.ForceLanguage;
    } else {
      switch (navigator.language) {
      case "zh-CN":
      case "zh":
        language = "zh";
        break;
      case "en":
      case "en-US":
        language = "en";
        break;
      default:
        language = Conf.DefaultLanguage;
      }
    }
  }

  return language;
}

i18n.use(initReactI18next).init({
  lng: initLanguage(),
  resources: resources,
  // The top-level keys of data.json are the namespaces, addressed as
  // "general:Name". nsSeparator has to be spelled out even though ":" is the
  // default: left implicit, i18next treats a key holding a space or a comma as
  // natural language and stops splitting the namespace off it, so every
  // multi-word key would render as its own raw "general:Display name" text.
  ns: Object.keys(en),
  defaultNS: "general",
  keySeparator: false,
  nsSeparator: ":",
  interpolation: {
    escapeValue: false,
  },
  saveMissing: true,
});

export default i18n;
