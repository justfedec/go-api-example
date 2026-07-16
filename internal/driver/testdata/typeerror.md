# A program with a type error

This fixture checks that diagnostics point at lines of the Markdown file,
not at the tangled program.

```inkdown
let n = 1
```

The block above is fine; the mistake is below (string + int):

```inkdown
let s = "x" + n
```
