# `--group`

```text
--group <tag>
```

Run against switches whose `tags` list contains `<tag>`.

## When to use

Building floors, roles (`access`, `core`), or any tag you define in YAML.

## Examples

```bash
negev --group access --config docs/examples/config.yaml
negev --group building-a --config docs/examples/config.yaml --write
```

## Interactions

Mutually exclusive with `--target` and `--all`. Empty match set is an error (exit `1`).
