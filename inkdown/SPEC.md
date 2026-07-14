# The Inkdown Language Specification

Version 1 (v1)

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

Builtin names (`print`, `len`, `range`, `push`, `str`, `int`, `float`,
`string`, `bool`) are ordinary identifiers that are predeclared; they cannot be
redeclared.

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

Lists nest: `[[int]]` is a list of lists of integers.

There are **no implicit conversions**. Mixing `int` and `float` in one
expression is a compile error; convert explicitly with `int(x)` and `float(x)`.

Zero values (relevant only for globals, section 6.2): `0`, `0.0`, `""`,
`false`, and the empty list.

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
values in v1: a function name may only appear in call position. A call to a
function without a return type has no value and may only be used as a
statement.

### 4.4 List literals

A non-empty list literal takes the type of its elements, which must all agree.
The empty literal `[]` has no type of its own; it is only valid where a list
type is known from context, i.e. in a declaration with a type annotation
(`var xs: [int] = []`), in an assignment to a list variable, as an argument to
a user function with a list parameter, or in a `return` from a function with a
list return type.

## 5. Statements

### 5.1 Declarations

```
let name = expr             # immutable binding, type inferred
let name: type = expr       # immutable binding, checked against annotation
var name = expr             # mutable variable, type inferred
var name: type = expr       # mutable variable, checked against annotation
```

An initializer is always required. `let` bindings cannot be reassigned, and
their contents cannot be mutated (`xs[i] = v` and `push(xs, v)` require a
`var` at the root). Redeclaring a name in the same scope is an error; an inner
block may shadow an outer name.

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
return type must end in a *terminating statement*: either a `return`, or an
`if`/`else` chain (with a final `else`) whose branches all terminate.

### 5.6 Expression statements

Only calls may stand alone as statements. Calls to value-returning **user**
functions may discard the result; calls to value-returning **builtins**
(`len`, `range`, `str`, `int`, `float`) may not.

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
value. Declarations inside a nested block (e.g. inside a top-level `if`) are
locals, not globals.

Function names, global names, and builtin names share one namespace; duplicates
are an error.

## 7. Builtins

| Builtin           | Signature                          | Notes |
| ----------------- | ---------------------------------- | ----- |
| `print(a, b, …)`  | any types, any count → (no value)  | prints arguments separated by spaces, then a newline; lists print Go-style, e.g. `[1 2 3]` |
| `len(x)`          | `string` or `[T]` → `int`          | for strings, length in bytes |
| `range(a, b)`     | `int, int → [int]`                 | the integers `a, a+1, …, b-1`; empty when `b <= a` |
| `push(xs, v)`     | `[T], T → (no value)`              | appends `v` to `xs`; `xs` must be an assignable list rooted in a `var` |
| `str(x)`          | any type → `string`                | decimal for `int`; shortest form for `float`; `true`/`false` for `bool`; identity for `string` |
| `int(x)`          | `int`, `float`, or `string` → `int`| truncates floats toward zero; panics if a string does not parse |
| `float(x)`        | `int`, `float`, or `string` → `float` | panics if a string does not parse |

## 8. Run-time behavior

Inkdown compiles to Go, so run-time failures are Go panics: index out of
range, integer division by zero, and failed `int()`/`float()` string parses.
The generated code carries `//line` directives, so panic stack traces point at
the original `.md` lines. A panic terminates the program with a nonzero exit
status.

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

expr        = orexpr ;
orexpr      = andexpr { "or" andexpr } ;
andexpr     = notexpr { "and" notexpr } ;
notexpr     = "not" notexpr | cmpexpr ;
cmpexpr     = addexpr { ( "==" | "!=" | "<" | "<=" | ">" | ">=" ) addexpr } ;
addexpr     = mulexpr { ( "+" | "-" ) mulexpr } ;
mulexpr     = unary { ( "*" | "/" | "%" ) unary } ;
unary       = "-" unary | postfix ;
postfix     = primary { "(" [ args ] ")" | "[" expr "]" } ;
args        = expr { "," expr } [ "," ] ;
primary     = INT | FLOAT | STRING | "true" | "false"
            | IDENT | listlit | "(" expr ")" ;
listlit     = "[" [ args ] "]" ;
```

(Only identifiers denoting functions may be called, and calls/indexing are the
only postfix forms; the parser accepts the general shape and the checker
enforces the rest.)

## 10. Diagnostics

Compile-time errors are reported as:

```
file.md:line:col: message
```

where `line` is a line of the original Markdown document. The compiler stops
at the first error.

## 11. Limitations and future directions

Deliberately out of scope for v1: maps and structs, first-class functions and
closures, string indexing/slicing, multi-file programs and imports, a standard
library beyond the builtins, multiple-error reporting, and named code-block
references (Knuth-style `<<chunk>>` macros). The design keeps the door open:
all of these fit the existing pipeline (extract → lex → parse → check → emit
Go).
