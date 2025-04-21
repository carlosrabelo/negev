# `--concurrency`

```text
--concurrency <n>
```

Maximum concurrent switch runs (worker pool size). Default: `4`. Must be `>= 1`.

## When to use

Tune load on the network or bastion when using `--all` / `--group` / multi `--target`.

## Examples

```bash
negev --all --config docs/examples/config.yaml --concurrency 2
negev --group access --config docs/examples/config.yaml --concurrency 8
```

## Notes

Each worker buffers its own stdout/stderr until the target finishes, then flushes with a `[label]` prefix.
