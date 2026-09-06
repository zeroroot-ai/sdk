# Security policy

## Reporting a vulnerability

**Do not open a public issue.**

Report privately through GitHub Security Advisories:
[Report a vulnerability](https://github.com/zeroroot-ai/sdk/security/advisories/new)

## What to expect

| | |
|---|---|
| Acknowledgement | within 3 working days |
| Initial assessment | within 10 working days |
| Fix or mitigation plan | communicated with the assessment |

If you have not heard back within 3 working days, assume the report did not
reach us and escalate through any other channel you have. Silence is a failure
on our side, not a decision.

## Scope

This repository is the component-development API: protos and Go types that
customer components are written against.

**The highest severity here is anything that lets a component escape the
contract it declares** — a manifest or proto shape that grants an agent
capability it did not ask for, or that lets a component read another tenant's
data through the SDK surface. This code runs inside other people's builds, so a
supply-chain defect here reaches further than a bug in the platform.

## Out of scope

- Findings in a deployment you control that come from your own configuration
- Anything requiring a privileged position we already assume hostile
- Automated scanner output with no demonstrated impact; show the path

## Safe harbour

We will not pursue or support legal action against anyone who reports in good
faith under this policy, stays within scope, and does not access, modify or
retain data belonging to anyone else.
