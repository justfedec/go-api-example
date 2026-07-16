# Chat with Claude

A terminal chat REPL in a dozen lines of Markdown. It needs the
`ANTHROPIC_API_KEY` environment variable; the model defaults to
`claude-opus-4-8` and can be overridden with `INKDOWN_LLM_MODEL`.

```inkdown
print("Chat with Claude — press Enter on an empty line to quit")
while true {
  print("")
  print("you:")
  let q = read_line()
  if q == "" { break }
  print("")
  print("claude:")
  print(llm.ask(q, "Keep answers short: three sentences at most."))
}
```

Each `llm.ask` call is independent — this little REPL has no memory. Passing
the running transcript back in as context is a nice exercise:

```inkdown example
var transcript = ""
let q = read_line()
transcript += "user: " + q + "\n"
let a = llm.ask(transcript, "Continue this conversation as the assistant.")
transcript += "assistant: " + a + "\n"
```
