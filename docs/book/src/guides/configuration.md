# Configuration

Everything ZeroClaw does is driven by its configuration (`config.toml`), and
the gateway exposes it field by field. Paths are dotted:
`providers.models.anthropic.default.api_key`, `agents.helper.model`,
`channels.whatsapp.main.allowed_groups`.

## Reading

- `Config().Get(ctx, path)` returns a `*PropValue`: `Value` holds the
  field's display value as `json.RawMessage`, or, for a secret
  (`IsSecret()`), only `Populated` says whether it is set; its value is never
  returned. `Warnings` lists validation problems.
- `Config().List(ctx, prefix)` enumerates every field under a prefix with its
  kind (`string`, `bool`, `integer`, `enum`, `alias-ref`, …), category,
  current value and whether an environment variable overrides it.
- `Config().Snapshot(ctx)` returns the whole configuration with secrets
  masked. It exists for compatibility; prefer the two calls above.
- `Config().Schema(ctx)` and `Config().PropSchema(ctx, path)` return the JSON
  Schema (public, static per gateway build).

## Writing

- `Config().Set(ctx, path, value, comment)` writes one field. `value` is any
  value `encoding/json` can encode; the gateway coerces it to the field's
  kind and refuses a mismatch with 400 `value_type_mismatch`. A map alias the
  path implies is created (`agents.new_agent.model` creates
  `agents.new_agent`). A non-empty `comment` is written above the key.
- `Config().Reset(ctx, path)` resets a field to its default.
- `Config().Patch(ctx, ops, overrideDrift)` applies several operations at
  once, all or nothing:

```go
_, err := client.Config().Patch(ctx, []zeroclaw.PatchOp{
	zeroclaw.TestOp("agents.helper.model", "gpt-5.4-mini"),
	zeroclaw.ReplaceOp("agents.helper.model", "gpt-5.5"),
	zeroclaw.CommentOp("agents.helper.model", "upgraded by the billing service"),
}, false)
if err != nil {
	return err
}
```

`AddOp` and `RemoveOp` complete the set. If a patched field was edited on
disk since the gateway loaded it, the patch is refused with 409
`config_changed_externally`; `Config().Drift(ctx)` shows what differs, and
`overrideDrift` set to `true` overwrites it.

## Aliases: agents, providers, channels

Agents and other named entries are map aliases:

- `MapKeys(ctx, "agents")` lists them;
- `CreateMapKey(ctx, "agents", "helper")` creates one with defaults;
- `RenameMapKey(ctx, "agents", "helper", "assistant")` renames it and every
  reference to it (and moves an agent's workspace, memory, cron jobs and
  sessions);
- `DeletePlan(ctx, "agents", "helper")` shows what a delete would block on or
  rewrite, and `DeleteMapKey` does it. Deleting an agent archives its
  workspace and exports its memory first.

`AliasValues(ctx, zeroclaw.AliasModelProviders)` lists the values an alias
reference may take, and `AgentOptions(ctx)` everything an agent can point to.

## Applying changes

Most writes take effect at once. Some subsystems pick changes up only on a
daemon reload: `Config().PendingReload(ctx)` says whether one is due, and
`System().Reload(ctx)` performs it (the connection drops briefly).

## Setup sections and catalogs

`Sections(ctx)`, `SectionItems(ctx, section)` and
`SelectSectionItem(ctx, section, key, alias)` drive the same setup flow as
the dashboard. `Catalog(ctx)` and `CatalogModels(ctx, provider)` list the
known model providers and their models (the latter may call the provider
with the configured key). `RefreshContextWindow(ctx, providerType, alias)`
stores a model's context window as the provider reports it.
