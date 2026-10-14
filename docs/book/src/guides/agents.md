# Agents, personality and skills

An agent is a configuration entry (`agents.<alias>`): create, rename and
delete agents with the [configuration](./configuration.md) calls. Its files
and skills are reached through `client.Agent(alias)`.

## Workspace

Each agent has a workspace directory. Paths are relative to it; leaving it is
refused, and the personality files and `sessions` at its root are protected.

```go
agent := client.Agent("helper")
listing, err := agent.ListFiles(ctx, "docs")
if err != nil {
	return err
}
for _, entry := range listing.Entries {
	fmt.Println(entry.Name, entry.Kind)
}
file, err := agent.ReadFile(ctx, "docs/catalog.pdf") // base64 when binary
if err != nil {
	return err
}
fmt.Println(file.Encoding, file.Size)
if err := agent.MakeDir(ctx, "archive"); err != nil {
	return err
}
if err := agent.MovePath(ctx, "docs/old.pdf", "archive/old.pdf"); err != nil {
	return err
}
if err := agent.RemovePath(ctx, "archive"); err != nil {
	return err
}
```

The shared directory (`<install>/shared/`) is listed, and has directories
made and removed, under `client.Files()`, which also uploads images into an
agent's workspace (`UploadImage`) and returns the `[IMAGE:…]` marker to
attach them to a chat message.

## Personality

`SOUL.md`, `IDENTITY.md`, `USER.md`, `AGENTS.md`, `TOOLS.md`, `HEARTBEAT.md`
and `MEMORY.md` shape an agent's behavior. `PersonalityFiles(ctx)` lists
them, `Personality(ctx, name)` reads one, and
`WritePersonality(ctx, name, content, expectedMtimeMs)` replaces it (20 000
characters at most). Pass the `MtimeMs` you read to make the write fail with
409 instead of overwriting someone else's change; `nil` writes regardless.
`PersonalityTemplates(ctx, zeroclaw.PersonalityContext{…})` renders the defaults without writing
them.

## Skills

Skills live in bundles (`skill_bundles.<alias>` in the configuration).
`client.Skills()` lists bundles and their skills and creates, reads, writes
and deletes (archives, or purges) a skill: front matter plus a Markdown body.
`client.Agent(alias).Skills(ctx)` shows what an agent actually loads, where
each skill comes from, and which the audit dropped.
