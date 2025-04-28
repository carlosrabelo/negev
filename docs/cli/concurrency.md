# `--concurrency`

```text
--concurrency <n>
```

Maximum concurrent switch runs (worker pool size). Default: `1`. Must be `>= 1`.

## When to use

Tune load on the network or bastion when using `--tag` or multi `--target`.

## Examples

```bash
negev --tag access --config docs/examples/config.yaml --concurrency 2
negev --tag building-a --config docs/examples/config.yaml --concurrency 8
```

## Notes

Each worker buffers its own stdout/stderr until the target finishes, then flushes with a `[label]` prefix.
