# Fibonacci

This program prints the first ten Fibonacci numbers. It is also this
program's documentation: Inkdown is a literate language, so the document and
the code are the same file, and the code blocks below are tangled together in
order before compiling.

## The function

The classic doubly recursive definition. `fib` takes an `int` and returns an
`int`; Inkdown functions declare their types explicitly:

```inkdown
func fib(n: int) -> int {
  if n < 2 { return n }
  return fib(n - 1) + fib(n - 2)
}
```

## The loop

`range(0, 10)` produces the integers 0 through 9, and `str` converts a number
to a string so we can build the label. Note that `fib` was already usable in
the block above — functions are hoisted, so blocks can come in whatever order
reads best:

```inkdown
for i in range(0, 10) {
  print("fib(" + str(i) + ") =", fib(i))
}
```

## A block that does not run

Fences tagged `inkdown example` are shown but never compiled — handy for
counterexamples. This one would recurse for a very long time:

```inkdown example
print(fib(1000))  # do not try this at home
```
