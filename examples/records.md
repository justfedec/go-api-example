# Records

A `record` groups named, typed fields into one value — the thing v1 forced
you to fake with parallel lists. Records are declared at the top level.

```inkdown
record Point {
  x: int
  y: int
}
```

Construct a value by naming every field (order is free, all are required),
read fields with a dot, and — for a `var` — assign through the dot:

```inkdown
var p = Point(x: 3, y: 4)
print(p.x, p.y)
p.x = p.x + 10
print(p)
```

Records nest, and lists of records work like any other list:

```inkdown
record Segment {
  from: Point
  to: Point
}

let seg = Segment(from: Point(x: 0, y: 0), to: p)
print(seg.to.x)

var points: [Point] = []
push(points, Point(x: 1, y: 1))
push(points, Point(x: 2, y: 4))
var sum = 0
for pt in points {
  sum += pt.y
}
print("points:", len(points), "sum of y:", sum)
```

`print` renders a record as `Name(field: value, ...)`; nested records show
in full, and a value is a reference, so two bindings can share one record:

```inkdown
print(seg)
var a = Point(x: 7, y: 7)
var b = a
b.x = 99
print(a.x)
```
