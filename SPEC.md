# The Inkdown Language Specification

Version 2 (v2)

v2 grows the language in exactly one place — namespaced builtin calls like
`http.get(...)` (§4.3) — and everything else through predeclared names: three
opaque handle types (§3.1) and a standard library of builtins for strings,
process access, HTTP (server and client), JSON, and LLM calls (§7). Programs
that avoided the new reserved names (§2.2) compile unchanged.

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
func return let var if else while for in
break continue and or not true false
```

Builtin names are predeclared and **reserved**: they cannot be redeclared,
shadowed, or assigned, and they cannot appear as values — a callable builtin
may only be called, and a type name may only appear in type position. The
reserved set is:

- the core builtins `print`, `eprint`, `len`, `range`, `push`, `str`, `int`,
  `float` and the type names `string`, `bool` (§7.1);
- the string builtins `split`, `join`, `contains`, `starts_with`, `index_of`,
  `substring`, `trim`, `lower`, `upper`, `replace` (§7.2);
- the process builtins `is_int`, `is_float`, `env`, `exit`, `read_line`
  (§7.3);
- the namespace roots `http`, `json`, `llm` (§4.3);
- the handle type names `server`, `request`, `response` (§3.1).

A `.` may appear only inside a namespaced builtin call (§4.3) or a float
literal.

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
| `server`, `request`, `response` | opaque handles (§3.1) | runtime pointer types |

Lists nest: `[[int]]` is a list of lists of integers.

**Lists are references.** Declaring or assigning a list copies the reference,
not the elements, so two bindings can alias the same list and mutations
through one are visible through the other. `let` restricts what may be done
*through that binding* (§5.1); it does not deep-freeze the list.

There are **no implicit conversions**. Mixing `int` and `float` in one
expression is a compile error; convert explicitly with `int(x)` and `float(x)`.

Zero values (relevant only for globals, section 6.2): `0`, `0.0`, `""`,
`false`, and the empty list. A handle-typed global read before its
declaration executes holds an **uninitialized handle** (§3.1).

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
| 8     | call `f(...)`, index `xs[i]` | see below               |          |

Binary operators of equal precedence associate to the left. `and` and `or`
short-circuit. `int / int` truncates toward zero, as in Go. Lists cannot be
compared with `==`.

`not x == y` parses as `not (x == y)`.

### 4.2 Indexing

`xs[i]` requires `xs` to be a list and `i` an `int`, and yields the element
type. Indexing out of range panics at run time. Strings cannot be indexed in
v1.

### 4.3 Calls

`f(a, b, c)` calls the function or builtin named `f`. Functions are not
values: a function name may only appear in call position. A call to a
function without a return type has no value and may only be used as a
statement.

Stdlib builtins live in the namespaces `http`, `json`, and `llm` and are
called with a dotted name: `http.get(url)`, `json.escape(s)`, `llm.ask(p)`.
The dot exists **only in call position**: `http.get` without an argument
list, a dot after anything but an identifier, and chained dots are all syntax
errors (grouping parentheses are transparent, so `(http).get(u)` parses like
`http.get(u)`). Namespace roots are reserved (§2.2) but user identifiers can
never contain a dot, so dotted names cannot collide with user code.

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
```

An initializer is always required. `let` bindings cannot be reassigned, and
they cannot be mutated **through that binding**: `xs[i] = v` and `push(xs, v)`
require a `var` at the root. Because lists are references (§3), a `let` list
is not deep-frozen — a `var` alias of the same list can still change the
elements the `let` binding sees. Copy the list if isolation matters.

Redeclaring a name in the same scope is an error; an inner block may shadow
an outer name. Unlike Go, a local that is never read is **not** an error.

### 5.2 Assignment

```
target = expr
target += expr    # also -= *= /= %=
```

`target` is a variable name or an index expression like `xs[i]` (nesting
allowed, `grid[i][j] = v`). The root variable must have been declared with
`var`. Compound assignments follow the typing rules of the corresponding
binary operator. Loop variables (section 5.4) are immutable.

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
(`len`, `str`, `http.get`, `llm.ask`, ...) may not.

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

Function names, global names, and builtin names share one namespace; duplicates
are an error.

## 7. Builtins

Failures split by intent throughout the library: **guards** (`is_int`,
`json.has`, `http.ok`) let programs test before acting, the HTTP client
reports failure through its response value, and everything else panics —
panics are for programmer errors, not expected conditions.

### 7.1 Core

| Builtin           | Signature                          | Notes |
| ----------------- | ---------------------------------- | ----- |
| `print(a, b, …)`  | any types, any count → (no value)  | prints arguments separated by spaces, then a newline; lists print Go-style, e.g. `[1 2 3]` |
| `eprint(a, b, …)` | any types, any count → (no value)  | like `print`, to standard error |
| `len(x)`          | `string` or `[T]` → `int`          | for strings, length in **bytes** (string positions elsewhere count runes, §7.2) |
| `range(a, b)`     | `int, int → [int]`                 | the integers `a, a+1, …, b-1`; empty when `b <= a` |
| `push(xs, v)`     | `[T], T → (no value)`              | appends `v` to `xs`; `xs` must be an assignable list rooted in a `var`, and its index expressions may not contain calls (they would be evaluated twice) |
| `str(x)`          | any non-handle type → `string`     | decimal for `int`; shortest form for `float`; `true`/`false` for `bool`; identity for `string`; lists render Go-style (`[1 2 3]`); handle operands are a compile error |
| `int(x)`          | `int`, `float`, or `string` → `int`| truncates floats toward zero; panics if a string does not parse (guard with `is_int`) |
| `float(x)`        | `int`, `float`, or `string` → `float` | panics if a string does not parse (guard with `is_float`) |

### 7.2 Strings

String **positions count runes** (characters), so `index_of` and `substring`
compose safely on UTF-8 text; only `len` is byte-based.

| Builtin                | Signature                        | Notes |
| ---------------------- | -------------------------------- | ----- |
| `split(s, sep)`        | `string, string → [string]`      | Go semantics: `split("", ",")` is `[""]` |
| `join(xs, sep)`        | `[string], string → string`      | |
| `contains(s, sub)`     | `string, string → bool`          | |
| `starts_with(s, p)`    | `string, string → bool`          | |
| `index_of(s, sub)`     | `string, string → int`           | rune index of the first occurrence, `-1` if absent |
| `substring(s, i, j)`   | `string, int, int → string`      | runes `i` to `j-1`; panics when the range is invalid |
| `trim(s)`              | `string → string`                | strips leading/trailing whitespace |
| `lower(s)` / `upper(s)`| `string → string`                | |
| `replace(s, old, new)` | `string, string, string → string`| replaces every occurrence |

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
makes `json.get`/`json.len` panic (`json.has` just returns false).

| Builtin               | Signature                    | Notes |
| --------------------- | ---------------------------- | ----- |
| `json.get(doc, path)` | `string, string → string`    | scalars in string form (convert with `int()`/`float()`); objects/arrays as compact JSON, ready to walk again; panics when the path is missing — guard with `json.has` |
| `json.has(doc, path)` | `string, string → bool`      | false for a missing path or an unparsable document |
| `json.len(doc, path)` | `string, string → int`       | length of the array at path; panics otherwise |
| `json.escape(s)`      | `string → string`            | `s` as a quoted JSON string literal, for building JSON by concatenation |

### 7.7 LLM (`llm.`)

| Builtin                  | Signature                      | Notes |
| ------------------------ | ------------------------------ | ----- |
| `llm.ask(prompt)`        | `string → string`              | asks Claude via the Anthropic Messages API and returns the reply text |
| `llm.ask(prompt, system)`| `string, string → string`      | same, with a system prompt |

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
answering its last request without truncating it. A `while true` accept loop
keeps the process alive; stop it from outside (Ctrl-C).

## 9. Grammar

Newline handling (section 2.4) is left implicit; `NL` means one or more
newlines or semicolons.

```ebnf
program     = { toplevel } ;
toplevel    = funcdecl | statement ;

funcdecl    = "func" IDENT "(" [ params ] ")" [ "->" type ] block ;
params      = param { "," param } [ "," ] ;
param       = IDENT ":" type ;
type        = IDENT | "[" type "]" ;
block       = "{" { statement } "}" ;

statement   = decl | assign | ifstmt | whilestmt | forstmt
            | "return" [ expr ] | "break" | "continue" | exprstmt ;
decl        = ( "let" | "var" ) IDENT [ ":" type ] "=" expr ;
assign      = target ( "=" | "+=" | "-=" | "*=" | "/=" | "%=" ) expr ;
target      = IDENT { "[" expr "]" } ;
ifstmt      = "if" expr block { "else" "if" expr block } [ "else" block ] ;
whilestmt   = "while" expr block ;
forstmt     = "for" IDENT "in" expr block ;
exprstmt    = expr ;                     (* must be a call, see 5.6 *)

qualname    = IDENT "." IDENT ;        (* namespaced builtins, call position only *)

expr        = orexpr ;
orexpr      = andexpr { "or" andexpr } ;
andexpr     = notexpr { "and" notexpr } ;
notexpr     = "not" notexpr | cmpexpr ;
cmpexpr     = addexpr { ( "==" | "!=" | "<" | "<=" | ">" | ">=" ) addexpr } ;
addexpr     = mulexpr { ( "+" | "-" ) mulexpr } ;
mulexpr     = unary { ( "*" | "/" | "%" ) unary } ;
unary       = "-" unary | postfix ;
postfix     = primary { "(" [ args ] ")" | "[" expr "]" | "." IDENT } ;
args        = expr { "," expr } [ "," ] ;
primary     = INT | FLOAT | STRING | "true" | "false"
            | IDENT | listlit | "(" expr ")" ;
listlit     = "[" [ args ] "]" ;
```

(Only identifiers — plain or `qualname` — may be called, and a `. IDENT`
postfix is accepted only on a plain identifier that is immediately called:
`http.get(x)` is a call of the qualified name, while a dot anywhere else is a
syntax error. Indexing composes freely; the checker enforces the remaining
rules.)

## 10. Diagnostics

Compile-time errors are reported as:

```
file.md:line:col: message
```

where `line` is a line of the original Markdown document (the CLI prefixes
the whole diagnostic with `inkdown: `). The compiler stops at the first
error.

## 11. Limitations and future directions

Deliberately out of scope for v2: maps and structs (JSON stays string-typed,
§7.6), first-class functions and closures (the server model needs none,
§7.4), string indexing/slicing syntax (`substring` covers it), multi-file
programs and imports, concurrency (the server is sequential by design),
recoverable-error bindings (`let x, err = f()` — today's split is guards +
`http.ok` + panics), multiple-error reporting, and named code-block
references (Knuth-style `<<chunk>>` macros). The design keeps the door open:
all of these fit the existing pipeline (extract → lex → parse → check → emit
Go) and the builtin registry.
