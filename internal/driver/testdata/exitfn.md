# regression: exit() as a function terminator

Both shapes must compile (SPEC §5.5): a body ending in `exit`, and an
`if`/`else` chain with an `exit` branch.

```inkdown
func pick(n: int) -> int {
  if n > 0 {
    return n
  } else {
    exit(4)
  }
}

func die(msg: string) -> int {
  eprint(msg)
  exit(3)
}

print(pick(7))
print(die("boom"))
```
