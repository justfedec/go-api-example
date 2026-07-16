# e2e fixture: llm.ask

The test points `ANTHROPIC_BASE_URL` at a fake Messages API server.

```inkdown
let answer = llm.ask("What is Inkdown?", "You are terse.")
print(answer)
```
