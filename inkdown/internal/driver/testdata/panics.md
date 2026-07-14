# A program that panics

Indexing out of range panics at run time, and the //line directives make the
Go stack trace point back at this document.

```inkdown
let xs = [1, 2]
print(xs[5])
```
