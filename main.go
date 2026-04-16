package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
}

var (
	todos  = []Todo{}
	mu     sync.RWMutex
	nextID = 1
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8080", "listen address")
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/todos", handleTodos)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Go API listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Go API</title></head><body>
<h1>Go Todo API</h1>
<p>Endpoints:</p>
<ul>
<li><a href="/todos">GET /todos</a> - List all todos</li>
<li>POST /todos - Create a todo (JSON body: {"title": "..."})</li>
<li><a href="/health">GET /health</a> - Health check</li>
</ul>
</body></html>`)
}

func handleTodos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		defer mu.RUnlock()
		json.NewEncoder(w).Encode(todos)

	case http.MethodPost:
		var input struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Title == "" {
			http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
			return
		}
		mu.Lock()
		todo := Todo{ID: nextID, Title: input.Title, CreatedAt: time.Now()}
		nextID++
		todos = append(todos, todo)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(todo)

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
