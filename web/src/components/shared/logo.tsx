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
import {cn} from "@/lib/utils";

// Two files with the ink baked in rather than one recoloured on the fly: an
// <img> cannot inherit currentColor, and the theme is a class on <html>, so
// prefers-color-scheme inside the SVG would not follow the toggle either.
export function Logo({className}: {className?: string}) {
  return (
    <>
      <img
        src={`${Setting.StaticBaseUrl}/img/apache-casbin-logo.svg`}
        alt="Apache Casbin"
        className={cn("dark:hidden", className)}
      />
      <img
        src={`${Setting.StaticBaseUrl}/img/apache-casbin-logo_dark.svg`}
        alt=""
        aria-hidden="true"
        className={cn("hidden dark:block", className)}
      />
    </>
  );
}

// The gopher on its own, for slots too narrow for the wordmark. It carries its
// own colour and reads on either background, so there is only one of it.
export function LogoMark({className}: {className?: string}) {
  return <img src={`${Setting.StaticBaseUrl}/img/casbin-mark.svg`} alt="Apache Casbin" className={className} />;
}
