# `INDEX.md` — template

At the **root** of the base, over every project in it. Paths are relative to the root, so they start
with the project directory — that is what makes one index readable across services.

```markdown
# Code knowledge index

| Project | Module | File | Last slice | Date |
|---|---|---|---|---|
| billing | src/payments | billing/modules/payments.md | 260813-payments-create | 2026-08-13 |
| lk-api  | src/payments | lk-api/modules/payments.md  | 260901-payments-proxy  | 2026-09-01 |

## Slices
| Project | Slice | Run | Task | Title | Type | Date | Modules |
|---|---|---|---|---|---|---|---|
| billing | 260813-payments-create | 260813142530 | PROJ-855 | ... | feature | 2026-08-13 | src/payments |
```

Two projects describing a module of the same name is normal and is exactly what the `Project` column
is for: `billing`'s payments and `lk-api`'s payments are different code. Only the row whose
`Project` is this run's may be touched.
