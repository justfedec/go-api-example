# The Inkdown Language Specification

Version 3 (v3)

v3 adds composite data and recoverable errors: **records** (§6.3), named-field
value types that replace parallel lists, and the two-name declaration
`let x, err = f(...)` (§5.1) that turns a fallible builtin's failure into a
value instead of a panic. It also finishes the string story — the string
builtins move into the `str.` namespace with a rune-based `str.len` and
`str.slice` (§7.2), and `len()` becomes list-only (§7.1) — and makes the
reference implementation fast by caching compiled binaries (§8).

**Breaking changes from v2:** the flat string builtins (`split`, `contains`,
`substring`, …) are now `str.split`, `str.contains`, `str.slice`, …;
`len(aString)` is a compile error (use `str.len`); and a dotted expression
that is not immediately called is now field access, not a namespaced call.

v2 grew the language in one place — namespaced builtin calls like
`http.get(...)` (§4.3) — plus opaque handle types (§3.1) and a standard
library (§7). v3 keeps that shape: records are the only new statement form,
and the only new expression forms are field selection `x.f` (§4.3) and the
two-name declaration.

Inkdown is a small, statically typed, literate programming language. An Inkdown
program **is** a Markdown document: prose is documentation, and only fenced code
blocks tagged `inkdown` are compiled. The reference compiler is written in pure
Go, transpiles Inkdown to Go, and uses the Go toolchain to produce native
binaries.

This document is the authoritative description of the language accepted by the
reference compiler in this repository.

## 1. Source files and the literate rule

An Inkdown source file is a UTF-8 Markdown file, conventionally with the `.md`
extension.

The compiler scans the document for *fenced code blocks* and ignores everything
else (headings, prose, lists, HTML, and code blocks in other languages).

A fenced code block participates in compilation when:

- The opening fence starts at column 0 (not indented) and consists of three or
  more backticks (` ``` `) or three or more tildes (`~~~`).
- The first word of the fence's info string is `inkdown`.
- The info string does **not** contain the word `example`. A block tagged
  `inkdown example` is rendered like any other code block but is invisible to
  the compiler — use it for snippets that should not run.

A block is closed by a line at column 0 consisting of at least as many fence
characters of the same kind as the opening fence (and nothing else but
whitespace). An unclosed block extends to the end of the file. Fences of the
other kind inside an open block are ordinary content.

All compiling blocks are concatenated top to bottom, in order of appearance,
into a single program (the literate "tangle"). A file with no `inkdown` blocks
is an error.

The compiler records the original Markdown line of every extracted line, and
all diagnostics point at the `.md` document, not at the concatenated program.

## 2. Lexical structure

### 2.1 Comments

A `#` inside a code block starts a comment that runs to the end of the line.
(Markdown headings live outside code blocks and are unaffected.)

### 2.2 Identifiers and keywords

Identifiers match `[A-Za-z_][A-Za-z0-9_]*`. The following keywords are
reserved and cannot be used as identifiers:

```
func record return let var if else while for in
break continue and or not true false
```

Builtin names are predeclared and **reserved**: they cannot be redeclared,
shadowed, or assigned, and they cannot appear as values — a callable builtin
may only be called, and a type name may only appear in type position. The
reserved set is:

- the core builtins `print`, `eprint`, `len`, `range`, `push`, `str`, `int`,
  `float` and the type names `string`, `bool` (§7.1);
- the process builtins `is_int`, `is_float`, `env`, `exit`, `read_line`
  (§7.3);
- the namespace roots `str`, `http`, `json`, `llm` (§4.3, §7.2);
- the handle type names `server`, `request`, `response` (§3.1).

The v2 flat string names (`split`, `contains`, `substring`, …) are **not**
reserved in v3 — they moved into the `str.` namespace (§7.2), freeing the bare
names for user code. Record type names (§6.3) are also reserved once declared.

A `.` may appear only inside a namespaced call (§4.3), a record field
selection (§4.3), or a float literal.

### 2.3 Literals

- **Integer**: decimal digits, e.g. `0`, `42`.
- **Float**: digits, a dot, digits, e.g. `3.14`, `0.5`. A leading or trailing
  bare dot (`.5`, `5.`) is not accepted.
- **String**: double quotes with escapes `\n`, `\t`, `\r`, `\\`, `\"`, e.g.
  `"hello\n"`. Strings cannot span lines.
- **Boolean**: the keywords `true` and `false`.
- **List**: `[expr, expr, ...]`, e.g. `[1, 2, 3]`. A trailing comma is
  allowed. See section 4.4 for typing of `[]`.

### 2.4 Statement termination

A newline terminates a statement. A semicolon `;` may also be used. Newlines
inside parentheses `(...)` and brackets `[...]` do not terminate statements, so
call arguments and list literals may span lines. To split a long expression
across lines, wrap it in parentheses.

An `else` may appear on the line after the closing `}` of the preceding block.

## 3. Types

| Inkdown  | Meaning                       | Go equivalent |
| -------- | ----------------------------- | ------------- |
| `int`    | signed integer                | `int`         |
| `float`  | 64-bit floating point         | `float64`     |
| `string` | immutable UTF-8 byte string   | `string`      |
| `bool`   | boolean                       | `bool`        |
| `[T]`    | list with elements of type T  | `[]T`         |
| record types (§6.3) | named-field values | pointer to a Go struct |
| `server`, `request`, `response` | opaque handles (§3.1) | runtime pointer types |

Lists nest: `[[int]]` is a list of lists of integers.

**Records are references** too (like lists): assigning or passing a record
copies the reference, and mutating a field through one binding is visible
through every alias. See §6.3.

**Lists are references.** Declaring or assigning a list copies the reference,
not the elements, so two bindings can alias the same list and mutations
through one are visible through the other. `let` restricts what may be done
*through that binding* (§5.1); it does not deep-freeze the list.

There are **no implicit conversions**. Mixing `int` and `float` in one
expression is a compile error; convert explicitly with `int(x)` and `float(x)`.

Zero values (relevant only for globals, section 6.2): `0`, `0.0`, `""`,
`false`, and the empty list. A handle-typed or record-typed global read before
its declaration executes holds an **uninitialized reference** (nil) — reading
a field of it, or passing it to a builtin, panics (§3.1, §6.3).

### 3.1 Handle types

`server`, `request`, and `response` are nominal, opaque handle types. Values
of these types are produced only by builtins (`http.serve`, `http.next`,
`http.get`, ...): there is no literal syntax, and no operator accepts them —
`==`, ordering, arithmetic, indexing, and `for` iteration are all compile
errors. `str()` rejects a handle operand; `print()` accepts one and renders
it as `<server>`, `<request>`, or `<response>`.

Handles may be used in type annotations, including inside lists (`[request]`
is legal, and `print`/`str` of such a list renders the `<...>` forms). An
**uninitialized handle** — a handle-typed global read before its declaration
statement has executed — panics with a self-describing message the moment it
is passed to any builtin; `print` renders it as `<nil>`.

## 4. Expressions

### 4.1 Operators and precedence

From lowest to highest binding strength:

| Level | Operators             | Operand types                  | Result   |
| ----- | --------------------- | ------------------------------ | -------- |
| 1     | `or`                  | `bool`                         | `bool`   |
| 2     | `and`                 | `bool`                         | `bool`   |
| 3     | `not` (unary)         | `bool`                         | `bool`   |
| 4     | `==` `!=`             | both `int`, `float`, `string`, or `bool` | `bool` |
| 4     | `<` `<=` `>` `>=`     | both `int`, `float`, or `string` | `bool` |
| 5     | `+` `-`               | both `int` or both `float`; `+` also concatenates `string` | operand type |
| 6     | `*` `/` `%`           | both `int` or both `float`; `%` is `int` only | operand type |
| 7     | `-` (unary)           | `int`, `float`                 | operand type |
| 8     | call `f(...)`, index `xs[i]`, field `x.f` | see below     |          |

Binary operators of equal precedence associate to the left. `and` and `or`
short-circuit. `int / int` truncates toward zero, as in Go. Lists and records
cannot be compared with `==`.

`not x == y` parses as `not (x == y)`.

### 4.2 Indexing

`xs[i]` requires `xs` to be a list and `i` an `int`, and yields the element
type. Indexing out of range panics at run time. Strings cannot be indexed
(use `str.slice`, §7.2).

### 4.3 Calls, namespaces, and field selection

`f(a, b, c)` calls the function, builtin, or record constructor named `f`.
Functions are not values: a function name may only appear in call position. A
call to a function without a return type has no value and may only be used as
a statement. Record constructors use **named arguments** (§6.3).

**Namespaced builtins.** Stdlib builtins live in the namespaces `str`, `http`,
`json`, and `llm` and are called with a dotted name immediately followed by an
argument list: `str.split(s, sep)`, `http.get(url)`, `llm.ask(p)`. The
namespace roots are reserved (§2.2).

**Field selection.** `x.f` reads field `f` of a record value `x` (§6.3). The
dot's meaning is decided by what follows it: an identifier namespace root
followed directly by `(` (`http.get(...)`) is a namespaced call; any other
`x.f` is a field selection. A field selection followed by `(` — i.e. a
method-style call — is an error (records have no methods).

### 4.4 List literals

A non-empty list literal takes the type of its elements, which must all agree.
The empty literal `[]` has no type of its own; it is only valid where a list
type is known from context:

- in a declaration with a type annotation (`var xs: [int] = []`);
- in an assignment to a list variable;
- as an argument to a user function — or a builtin (§7) — with a list
  parameter;
- in a `return` from a function with a list return type;
- as the second argument of `push` into a list of lists;
- as a non-first element of a list literal whose earlier elements fixed the
  type (`[[1], []]`).

## 5. Statements

### 5.1 Declarations

```
let name = expr             # immutable binding, type inferred
let name: type = expr       # immutable binding, checked against annotation
var name = expr             # mutable variable, type inferred
var name: type = expr       # mutable variable, checked against annotation
let name, err = call        # two-name form: bind a fallible builtin's error
```

An initializer is always required. `let` bindings cannot be reassigned, and
they cannot be mutated **through that binding**: `xs[i] = v`, `x.f = v`, and
`push(xs, v)` require a `var` at the root. Because lists and records are
references (§3), a `let` is not deep-frozen — a `var` alias can still change
the contents the `let` binding sees. Copy if isolation matters.

Redeclaring a name in the same scope is an error; an inner block may shadow
an outer name. Unlike Go, a local that is never read is **not** an error.

**The two-name form** `let name, err = call` (also `var`) binds the value of a
**fallible builtin** to `name` and its failure message to `err`, a `string`
that is `""` on success. It replaces a panic with a value, so untrusted input
can be handled inline:

```
let n, err = int(field)          # err != "" instead of panicking on bad input
if err != "" { ... }
let reply, aerr = llm.ask(prompt)  # a network failure is recoverable
```

The initializer must be a call to a fallible builtin: `json.get`, `json.len`,
`llm.ask`, or `int`/`float` **on a string argument** (the numeric conversions
cannot fail otherwise). The two-name form takes no type annotation. The
one-name form of the same builtins keeps panicking on failure (§8), so a
program opts into recovery per call site. `err` is an ordinary binding;
re-declaring it in one scope is an error, as always.

### 5.2 Assignment

```
target = expr
target += expr    # also -= *= /= %=
```

`target` is a variable name, an index expression like `xs[i]`, or a field
selection like `x.f` (nesting allowed: `grid[i][j] = v`, `todos[i].done = v`).
The root variable must have been declared with `var`. Compound assignments
follow the typing rules of the corresponding binary operator. Loop variables
(section 5.4) are immutable.

### 5.3 Conditionals

```
if cond {
  ...
} else if cond2 {
  ...
} else {
  ...
}
```

Conditions must be `bool`. There is no truthiness.

### 5.4 Loops

```
while cond {
  ...
}

for x in listExpr {
  ...
}
```

`for` iterates over the elements of a list; the loop variable is a new
immutable binding scoped to the body, typed as the element type. `break` and
`continue` behave as in Go and are only valid inside a loop.

### 5.5 Return

`return expr` in a function with a return type; bare `return` in a function
without one. `return` is not allowed at the top level. A function with a
return type must end in a *terminating statement*: a `return`, a call to
`exit(...)` (§7.3), or an `if`/`else` chain (with a final `else`) whose
branches all terminate. Nothing else counts — in particular, a trailing
`while true { ... }` does **not** satisfy the rule even though the function
can never fall through it; end such a function with an unreachable `return`.

### 5.6 Expression statements

Only calls may stand alone as statements. Calls to value-returning **user**
functions may discard the result; calls to value-returning **builtins**
(`len`, `str`, `http.get`, `llm.ask`, ...) and **record constructors** may
not.

## 6. Program structure

### 6.1 Functions

```
func name(param: type, param2: type) -> rettype {
  ...
}
```

The `-> rettype` clause is optional. Functions may only be declared at the top
level, in any block and in any order; every function is visible everywhere
(hoisting), so mutual recursion works. Parameters are immutable.

### 6.2 Top-level code and globals

Statements at the top level form the program's entry point and run top to
bottom, in the order the code blocks appear in the document.

A `let` or `var` declared directly at the top level is a **global**, visible
inside every function. Top-level code cannot mention a global before the line
that declares it. Because functions may run before a later global's
declaration statement has executed, a global read at that point holds its zero
value (§3) — for handle types that is an uninitialized handle, which panics
when passed to a builtin (§3.1). Declarations inside a nested block (e.g.
inside a top-level `if`) are locals, not globals.

Function names, global names, record type names, and builtin names share one
namespace; duplicates are an error.

### 6.3 Records

A `record` declaration introduces a nominal type with named, typed fields:

```
record Todo {
  id: int
  title: string
  done: bool
}
```

Records are declared only at the top level; like functions, they are visible
everywhere (a field type may name any record, including the one being declared
or one declared later, so recursive and mutually recursive records are legal).
Fields are separated by newlines and/or commas. A field may not be named
`String`. A record name is reserved once declared.

**Construction** uses named arguments — every field exactly once, in any
order, all required (there are no defaults):

```
let t = Todo(id: 1, title: "buy milk", done: false)
```

Named arguments are only for constructors; a positional constructor call, a
missing or unknown field, or a duplicate field is a compile error.

**Field access** reads a field: `t.title`. Through a `var` root, a field is
assignable: `t.done = not t.done` (§5.2). Records are **references** (§3): an
assignment or a `push` copies the reference, so mutating a field is visible
through every alias.

Records have **no methods** — a method-style call `t.f(...)` is an error.
Records cannot be compared with `==`. `print` renders a record as
`Name(field: value, ...)`, recursing into nested records up to a small depth
(beyond it, and for a cyclic record, nested records render as `Name(...)`);
`str` of a record is not allowed. A record-typed global read before its
declaration executes is nil, and reading a field of nil panics against the
source line (§8).

## 7. Builtins

Failures split by intent throughout the library. **Guards** (`is_int`,
`json.has`, `http.ok`) let programs test before acting; the **two-name
declaration** `let x, err = f(...)` (§5.1) turns a fallible builtin's failure
into a value; the HTTP client reports failure through its response value; and
everything else panics — panics are for programmer errors, not expected
conditions. The builtins marked *fallible* below are exactly those usable in
the two-name form.

### 7.1 Core

| Builtin           | Signature                          | Notes |
| ----------------- | ---------------------------------- | ----- |
| `print(a, b, …)`  | any types, any count → (no value)  | prints arguments separated by spaces, then a newline; lists print Go-style, e.g. `[1 2 3]` |
| `eprint(a, b, …)` | any types, any count → (no value)  | like `print`, to standard error |
| `len(x)`          | `[T] → int`                        | list elements only; `len(aString)` is a compile error — use `str.len` (§7.2) |
| `range(a, b)`     | `int, int → [int]`                 | the integers `a, a+1, …, b-1`; empty when `b <= a` |
| `push(xs, v)`     | `[T], T → (no value)`              | appends `v` to `xs`; `xs` must be an assignable list rooted in a `var`, and its index expressions may not contain calls (they would be evaluated twice) |
| `str(x)`          | any non-handle, non-record type → `string` | decimal for `int`; shortest form for `float`; `true`/`false` for `bool`; identity for `string`; lists render Go-style (`[1 2 3]`); handle and record operands are a compile error (records `print` fine) |
| `int(x)`          | `int`, `float`, or `string` → `int`| truncates floats toward zero; **fallible** on a string (guard with `is_int`, or use the two-name form) |
| `float(x)`        | `int`, `float`, or `string` → `float` | **fallible** on a string (guard with `is_float`, or use the two-name form) |

### 7.2 Strings (`str.`)

Every `str.` position counts **runes** (characters), not bytes, so `str.len`,
`str.slice`, and `str.index_of` compose safely on UTF-8 text —
`str.slice(s, 0, str.len(s))` is always valid. (`len()` is list-only, §7.1.)

| Builtin                  | Signature                        | Notes |
| ------------------------ | -------------------------------- | ----- |
| `str.len(s)`             | `string → int`                   | length in **runes** |
| `str.slice(s, i, j)`     | `string, int, int → string`      | runes `i` to `j-1`; panics when the range is invalid |
| `str.split(s, sep)`      | `string, string → [string]`      | Go semantics: `str.split("", ",")` is `[""]` |
| `str.join(xs, sep)`      | `[string], string → string`      | |
| `str.contains(s, sub)`   | `string, string → bool`          | |
| `str.starts_with(s, p)`  | `string, string → bool`          | |
| `str.index_of(s, sub)`   | `string, string → int`           | rune index of the first occurrence, `-1` if absent |
| `str.trim(s)`            | `string → string`                | strips leading/trailing whitespace |
| `str.lower(s)` / `str.upper(s)` | `string → string`         | |
| `str.replace(s, old, new)` | `string, string, string → string`| replaces every occurrence |

(`str` is also the conversion builtin `str(x)`, §7.1 — the bare name converts,
the dotted names are string operations.)

### 7.3 Process

| Builtin        | Signature            | Notes |
| -------------- | -------------------- | ----- |
| `is_int(s)`    | `string → bool`      | true iff `int(s)` would succeed |
| `is_float(s)`  | `string → bool`      | true iff `float(s)` would succeed |
| `env(name)`    | `string → string`    | environment variable, `""` when unset |
| `exit(code)`   | `int → (no value)`   | terminates with the given exit code; counts as a terminating statement (§5.5) |
| `read_line()`  | `→ string`           | next stdin line without its terminator; `""` at end of input |

### 7.4 HTTP server (`http.`)

The server is **blocking and sequential**: `http.serve` starts listening,
`http.next` blocks until a request arrives, and the program handles exactly
one request at a time. Every request must be answered with `http.respond`
(exactly once); an unanswered request leaves its client waiting forever.
Concurrent requests queue. See §8 for runtime details.

| Builtin                          | Signature                        | Notes |
| -------------------------------- | -------------------------------- | ----- |
| `http.serve(port)`               | `int → server`                   | panics if the port cannot be bound; the port is listening when it returns |
| `http.next(srv)`                 | `server → request`               | blocks for the next request |
| `http.method(req)`               | `request → string`               | `"GET"`, `"POST"`, ... |
| `http.path(req)`                 | `request → string`               | URL path, e.g. `/todos/1` |
| `http.body(req)`                 | `request → string`               | request body |
| `http.header(req, name)`         | `request, string → string`       | `""` when absent |
| `http.query(req, name)`          | `request, string → string`       | query parameter, `""` when absent |
| `http.set_header(req, n, v)`     | `request, string, string → (no value)` | must precede `http.respond`; panics after it |
| `http.respond(req, status, body)`| `request, int, string → (no value)` | answers and releases the request; panics if already answered |

### 7.5 HTTP client (`http.`)

The client **never panics on network weather** (a malformed header string is
a programmer error and does panic). `http.ok` is true iff a **complete**
response was received: on failure before any response arrives, status is `0`
and the text is the error message; if the headers arrived but the body could
not be read, `http.ok` is false while `http.status` keeps the received code.
HTTP-level errors are ordinary responses — check `http.status`. Redirects are
followed automatically (up to 10); requests time out after 10 minutes.

| Builtin                          | Signature                            | Notes |
| -------------------------------- | ------------------------------------ | ----- |
| `http.get(url)`                  | `string → response`                  | |
| `http.post(url, body)`           | `string, string → response`          | sends `Content-Type: application/json` |
| `http.request(m, url, hs, body)` | `string, string, [string], string → response` | headers as `"Name: value"` strings; a malformed entry panics |
| `http.status(resp)`              | `response → int`                     | `0` on transport failure |
| `http.text(resp)`                | `response → string`                  | response body (the error message on transport failure) |
| `http.ok(resp)`                  | `response → bool`                    | true iff an HTTP response was received |

### 7.6 JSON (`json.`)

JSON stays a string; `json.get` walks it with a dot-separated path — object
keys by name, array elements by index (`"todos.0.title"`; `""` is the root).
Numbers come back **exactly as written in the document** (no scientific
notation, no precision loss on large integers). Two limitations: object keys
that themselves contain a dot cannot be addressed, and an unparsable document
makes the one-name `json.get`/`json.len` panic (`json.has` just returns
false). `json.get` and `json.len` are **fallible**: the two-name form
(§5.1) recovers both a missing path and an unparsable document as an error
string — the shape to use for untrusted request bodies.

| Builtin               | Signature                    | Notes |
| --------------------- | ---------------------------- | ----- |
| `json.get(doc, path)` | `string, string → string`    | **fallible**; scalars in string form (convert with `int()`/`float()`); objects/arrays as compact JSON, ready to walk again; one-name form panics on a missing path — guard with `json.has` |
| `json.has(doc, path)` | `string, string → bool`      | false for a missing path or an unparsable document |
| `json.len(doc, path)` | `string, string → int`       | **fallible**; length of the array at path |
| `json.escape(s)`      | `string → string`            | `s` as a quoted JSON string literal, for building JSON by concatenation |

### 7.7 LLM (`llm.`)

| Builtin                  | Signature                      | Notes |
| ------------------------ | ------------------------------ | ----- |
| `llm.ask(prompt)`        | `string → string`              | **fallible**; asks Claude via the Anthropic Messages API and returns the reply text |
| `llm.ask(prompt, system)`| `string, string → string`      | same, with a system prompt |

`llm.ask` is **fallible**: the one-name form panics on any failure (missing
key, non-200 status, refusal, token-budget truncation); the two-name form
(§5.1) recovers it as an error string, so a network blip need not crash a
long-running program.

`llm.ask` panics with a self-describing message when `ANTHROPIC_API_KEY` is
unset, the API returns a non-200 status, the model declines the request, or
the reply is cut off by the token budget.
Configuration comes from the environment: `ANTHROPIC_API_KEY` (required),
`INKDOWN_LLM_MODEL` (default `claude-opus-4-8`), `INKDOWN_LLM_MAX_TOKENS`
(default `16000`), and `ANTHROPIC_BASE_URL` (default
`https://api.anthropic.com` — point it at a fake server for hermetic tests).
Never write an API key into a program: Inkdown documents are meant to be
published.

## 8. Run-time behavior

Inkdown compiles to Go, so run-time failures are Go panics: index out of
range, integer division by zero, failed `int()`/`float()` string parses, and
the library panics of §7. The generated code carries `//line` directives, so
panic stack traces point at the original `.md` lines. A panic terminates the
program with exit status 2; `exit(n)` terminates with `n`.

The compiled program depends only on the Go standard library — `http.` and
`llm.` included — so builds are hermetic and binaries are self-contained.

**Server model.** `http.serve` binds its port before returning and accepts
connections in the background; incoming requests are held — a buffered queue
of 64 plus one goroutine per additional connection — until the program takes
them with `http.next`. Handling is strictly sequential — there is no
concurrency in the language — and a response is fully flushed to the client
before `http.respond` returns, so a program may exit immediately after
answering its last request without truncating it. Each request must be
answered exactly once: calling `http.next` again before answering the
previous request panics. The server applies connection-level timeouts
(header 10s, body 1m, idle 2m) but no per-response write deadline, so a
request queued behind a slow handler is never dropped for waiting. A
`while true` accept loop keeps the process alive; stop it from outside
(Ctrl-C).

**Binary cache.** `inkdown run` caches the compiled native binary, keyed on
the generated Go and the Go toolchain, so a repeat run of an unchanged
program skips the `go build` step. The cache lives under `INKDOWN_CACHE_DIR`
(or the user cache directory); `--no-cache` forces a recompile, and any
change to the program, the compiler, or the toolchain invalidates the entry
automatically. Caching is an optimization only — it never changes a program's
behavior, and a missing cache directory just falls back to compiling each
run.

## 9. Grammar

Newline handling (section 2.4) is left implicit; `NL` means one or more
newlines or semicolons.

```ebnf
program     = { toplevel } ;
toplevel    = funcdecl | recorddecl | statement ;

funcdecl    = "func" IDENT "(" [ params ] ")" [ "->" type ] block ;
params      = param { "," param } [ "," ] ;
param       = IDENT ":" type ;
recorddecl  = "record" IDENT "{" field { fieldsep field } "}" ;
field       = IDENT ":" type ;
fieldsep    = NL | "," ;
type        = IDENT | "[" type "]" ;
block       = "{" { statement } "}" ;

statement   = decl | assign | ifstmt | whilestmt | forstmt
            | "return" [ expr ] | "break" | "continue" | exprstmt ;
decl        = ( "let" | "var" ) IDENT [ "," IDENT | ":" type ] "=" expr ;
assign      = target ( "=" | "+=" | "-=" | "*=" | "/=" | "%=" ) expr ;
target      = IDENT { "[" expr "]" | "." IDENT } ;
ifstmt      = "if" expr block { "else" "if" expr block } [ "else" block ] ;
whilestmt   = "while" expr block ;
forstmt     = "for" IDENT "in" expr block ;
exprstmt    = expr ;                     (* must be a call, see 5.6 *)

expr        = orexpr ;
orexpr      = andexpr { "or" andexpr } ;
andexpr     = notexpr { "and" notexpr } ;
notexpr     = "not" notexpr | cmpexpr ;
cmpexpr     = addexpr { ( "==" | "!=" | "<" | "<=" | ">" | ">=" ) addexpr } ;
addexpr     = mulexpr { ( "+" | "-" ) mulexpr } ;
mulexpr     = unary { ( "*" | "/" | "%" ) unary } ;
unary       = "-" unary | postfix ;
postfix     = primary { "(" [ args ] ")" | "[" expr "]" | "." IDENT } ;
args        = arg { "," arg } [ "," ] ;
arg         = [ IDENT ":" ] expr ;       (* named form: record constructors *)
primary     = INT | FLOAT | STRING | "true" | "false"
            | IDENT | listlit | "(" expr ")" ;
listlit     = "[" [ exprargs ] "]" ;
exprargs    = expr { "," expr } [ "," ] ;
```

Postfix disambiguation: `IDENT "." IDENT "("` where the first identifier is a
namespace root is a namespaced call (folded to one qualified name);
`x "." IDENT` otherwise is a field selection; a field selection followed by
`(` is rejected (records have no methods). Named arguments (`IDENT ":" expr`)
are accepted by the grammar for any call but restricted to record
constructors by the checker; the two-name `decl` is restricted to fallible
builtins by the checker.

## 10. Diagnostics

Compile-time errors are reported as:

```
file.md:line:col: message
```

where `line` is a line of the original Markdown document (the CLI prefixes
the whole diagnostic with `inkdown: `). The compiler stops at the first
error.

## 11. Limitations and future directions

v3 added records (§6.3) and recoverable error bindings (§5.1). Still
deliberately out of scope: maps (JSON stays string-typed, §7.6), record
methods and first-class functions/closures (the server model needs none),
generics (so no user-defined `map`/`filter`/`sort`), multi-file programs and
imports, concurrency (the server is sequential by design), multiple-error
reporting, and named code-block references (Knuth-style `<<chunk>>` macros).
The design keeps the door open: all of these fit the existing pipeline
(extract → lex → parse → check → emit Go) and the builtin registry.
