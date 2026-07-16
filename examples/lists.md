# Working with lists

A tour of Inkdown's lists, mutation rules, and conversions. Like every
Inkdown program, the prose you are reading is the program's documentation and
the code blocks are the program.

## Setup

A `let` binding is immutable; a `var` is mutable. Globals declared at the top
level are visible inside functions, which is how `banner` reaches `greet`:

```inkdown
let banner = "== lists demo =="

func greet() {
  print(banner)
}

greet()
```

## Building a list

An empty list needs a type annotation (`[]` alone says nothing about its
elements). `push` appends in place, and only works on `var` lists:

```inkdown
func square(n: int) -> int {
  return n * n
}

var squares: [int] = []
for n in range(1, 6) {
  push(squares, square(n))
}

print("squares:", squares)
print("count:", len(squares))
print("first:", squares[0], "last:", squares[len(squares) - 1])
```

## Reducing it

A `while` loop and a couple of `var`s compute the sum; `float` conversions
keep the average exact. Inkdown never mixes `int` and `float` silently:

```inkdown
var total = 0
var i = 0
while i < len(squares) {
  total += squares[i]
  i += 1
}
print("total:", total)
print("average:", float(total) / float(len(squares)))
```

## Formatting it

Strings concatenate with `+`, and `str` converts values explicitly:

```inkdown
var csv = ""
for s in squares {
  if csv != "" {
    csv += ", "
  }
  csv += str(s)
}
print("csv: " + csv)
```
