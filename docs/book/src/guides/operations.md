# Cron, memory and cost

## Cron jobs

A job runs a shell command or an agent prompt on a schedule and can deliver
its output to a channel:

```go
delivery := zeroclaw.AnnounceDelivery("whatsapp.main", "+34600000000")
job, err := client.Cron().Create(ctx, &zeroclaw.NewCronJob{
	Agent:    "helper",
	Schedule: "0 9 * * 1-5",
	Tz:       "Europe/Madrid",
	Prompt:   "Summarize yesterday's orders",
	Delivery: &delivery,
})
if err != nil {
	return err
}
result, err := client.Cron().Run(ctx, job.ID) // runs it now and waits
if err != nil {
	return err
}
fmt.Println(result.Success, result.Output)
```

`Update` changes a job (pause with `&zeroclaw.CronJobPatch{Enabled: &off}`,
where `off := false`), `Runs` lists its history, and `Settings` /
`UpdateSettings` cover the scheduler. Optional fields are pointers or
omitted when empty.

A shell job with `ShellOutputFormat: "raw"` and a `Delivery` announces its
output verbatim: a way to post a fixed message (or a `[DOCUMENT:…]` marker)
on a channel through an agent's channel binding.

## Memory

`Memory().Search(ctx, zeroclaw.MemoryQuery{…})` recalls entries by text or
time (at most 50), or lists a category; `Store` and `Forget` write. Each
agent can have its own memory (`Agent`) besides the shared one.

## Cost

`Cost().Summary(ctx, from, to)` returns tokens, calls and USD in a window,
by model and by agent; `Cost().Agent(ctx, alias)` an agent's session and
daily figures. Models without known rates count as free unless their prices
are configured.

## The gateway itself

`client.System()` covers health (`Health`, public), `Status`, Prometheus
`Metrics`, `Doctor`, `Tools`, `Integrations`, host `CLITools`, connected
terminal clients (`Tuis`), paged structured `Logs`, version checks and
self-upgrades, and the daemon's `Reload` and `Shutdown`.
