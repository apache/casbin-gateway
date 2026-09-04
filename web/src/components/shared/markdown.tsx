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

import ReactMarkdown, {type Components} from "react-markdown";
import remarkGfm from "remark-gfm";

import {cn} from "@/lib/utils";

/**
 * Every element the renderer can emit, styled here rather than by a typography
 * plugin: an instruction file is read at the size the rest of the app is read
 * at, and raw HTML inside one is left as text, which is what react-markdown
 * does without a raw plugin.
 */
const components: Components = {
  h1: ({children}) => <h1 className="mt-4 mb-2 text-lg font-semibold first:mt-0">{children}</h1>,
  h2: ({children}) => <h2 className="mt-4 mb-2 text-base font-semibold first:mt-0">{children}</h2>,
  h3: ({children}) => <h3 className="mt-3 mb-1.5 text-sm font-semibold first:mt-0">{children}</h3>,
  h4: ({children}) => <h4 className="mt-3 mb-1.5 text-sm font-medium first:mt-0">{children}</h4>,
  p: ({children}) => <p className="my-2 leading-relaxed first:mt-0 last:mb-0">{children}</p>,
  ul: ({children}) => <ul className="my-2 list-disc space-y-1 pl-5">{children}</ul>,
  ol: ({children}) => <ol className="my-2 list-decimal space-y-1 pl-5">{children}</ol>,
  li: ({children}) => <li className="leading-relaxed">{children}</li>,
  a: ({children, href}) => (
    <a href={href} target="_blank" rel="noreferrer" className="text-primary underline underline-offset-2">
      {children}
    </a>
  ),
  blockquote: ({children}) => (
    <blockquote className="text-muted-foreground my-2 border-l-2 pl-3">{children}</blockquote>
  ),
  hr: () => <hr className="my-3" />,
  // A fenced block arrives as <pre><code>, so only the inline case is styled
  // here and the block keeps the pre's own scrolling.
  code: ({children, className}) =>
    (className ?? "").startsWith("language-") ? (
      <code className={className}>{children}</code>
    ) : (
      <code className="bg-muted rounded px-1.5 py-0.5 font-mono text-xs">{children}</code>
    ),
  pre: ({children}) => (
    <pre className="bg-muted scrollbar-thin my-2 overflow-x-auto rounded-md p-3 font-mono text-xs">
      {children}
    </pre>
  ),
  table: ({children}) => (
    <div className="scrollbar-thin my-2 overflow-x-auto rounded-md border">
      <table className="w-full text-left text-xs">{children}</table>
    </div>
  ),
  th: ({children}) => <th className="bg-muted/50 border-b px-2 py-1.5 font-medium">{children}</th>,
  td: ({children}) => <td className="border-b px-2 py-1.5 last:border-0">{children}</td>,
  img: ({src, alt}) => <img src={typeof src === "string" ? src : undefined} alt={alt} className="my-2 max-w-full rounded" />,
};

/** Markdown as the agent's own reader would lay it out, GitHub flavour. */
export function Markdown({content, className}: {content: string; className?: string}) {
  return (
    <div className={cn("text-sm break-words", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
