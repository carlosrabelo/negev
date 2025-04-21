# `--all`

```text
--all
```

Run against every switch listed in the loaded YAML.

## When to use

Fleet-wide sandbox or write runs.

## Examples

```bash
negev --all --config docs/examples/config.yaml
negev --all --config docs/examples/config.yaml --concurrency 4 --write
```

## Interactions

Mutually exclusive with `--target` and `--group`.

## Notes

Output is buffered per target; a summary table prints at the end. Exit `1` if any target fails.
