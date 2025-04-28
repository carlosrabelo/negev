# `--tag`

```text
--tag <tag>
```

Run against switches whose YAML `tag` field (string or list) contains `<tag>`.

## When to use

Group switches by site, branch, building floor, or role (`access`, `core`) in YAML without selecting every switch globally.

## Examples

```bash
negev --tag access --config docs/examples/config.yaml
negev --tag building-a --config docs/examples/config.yaml --write
```

## Interactions

Mutually exclusive with `--target`. Empty match set is an error (exit `1`). Legacy YAML `tags:` lists are also accepted and merged.
