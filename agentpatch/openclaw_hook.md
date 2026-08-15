---
name: casbin-gateway-agent-monitor
description: "Stream OpenClaw activity to Casbin Gateway"
metadata:
  {
    "openclaw":
      {
        "emoji": "📋",
        "events":
          [
            "command",
            "message",
            "session:patch",
            "session:compact:before",
            "session:compact:after",
            "agent:bootstrap",
            "gateway:startup",
          ],
      },
  }
---

# Casbin Gateway Agent Monitor

This hook sends OpenClaw activity to the local Casbin Gateway monitoring
timeline. It records activity only and never changes an OpenClaw action.
