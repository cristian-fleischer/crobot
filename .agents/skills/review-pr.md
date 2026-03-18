IMPORTANT: Before performing this code review, you MUST run the following
command and read its output carefully:

```bash
crobot review-instructions
```

The output contains the review methodology, required JSON schema, severity
guidelines, workflow steps, and rules that you MUST follow exactly. Do not
skip this step — CRoBot requires findings in a specific format and workflow
order.

After reading and understanding the instructions, perform a code review
following the workflow described.

If arguments are provided ($ARGUMENTS), review that specific PR.
If no arguments are provided, perform a local review of unpushed changes
using the `export_local_context` MCP tool, or run `crobot review` with no
PR argument.
