# regression: printing a cyclic record must not overflow the stack

```inkdown
record Node {
  val: int
  next: [Node]
}
var a = Node(val: 1, next: [])
var b = Node(val: 2, next: [])
push(a.next, b)
push(b.next, a)
print(a)
```
