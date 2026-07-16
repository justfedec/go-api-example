package stdlib

import "github.com/justfedec/inkdown/internal/types"

// The http namespace: a blocking, sequential web server (no callbacks — the
// program pulls requests off a queue with http.next and answers each with
// http.respond) and a non-panicking HTTP client (transport failures surface
// through http.ok/http.status, never as panics).
var (
	Server   = &types.Opaque{Name: "server"}
	Request  = &types.Opaque{Name: "request"}
	Response = &types.Opaque{Name: "response"}
)

func init() {
	Roots["http"] = true
	Types["server"] = Server
	Types["request"] = Request
	Types["response"] = Response
	TypeChunks["server"] = "httpserver"
	TypeChunks["request"] = "httpserver"
	TypeChunks["response"] = "httpclient"

	str := types.String
	strList := &types.List{Elem: types.String}

	register(
		// Server.
		&Spec{Name: "http.serve", Params: []types.Type{types.Int}, Ret: Server, GoFunc: "_ink_serve", Chunk: "httpserver"},
		&Spec{Name: "http.next", Params: []types.Type{Server}, Ret: Request, GoFunc: "_ink_next", Chunk: "httpserver"},
		&Spec{Name: "http.method", Params: []types.Type{Request}, Ret: str, GoFunc: "_ink_method", Chunk: "httpserver"},
		&Spec{Name: "http.path", Params: []types.Type{Request}, Ret: str, GoFunc: "_ink_path", Chunk: "httpserver"},
		&Spec{Name: "http.body", Params: []types.Type{Request}, Ret: str, GoFunc: "_ink_body", Chunk: "httpserver"},
		&Spec{Name: "http.header", Params: []types.Type{Request, str}, Ret: str, GoFunc: "_ink_header", Chunk: "httpserver"},
		&Spec{Name: "http.query", Params: []types.Type{Request, str}, Ret: str, GoFunc: "_ink_query", Chunk: "httpserver"},
		&Spec{Name: "http.set_header", Params: []types.Type{Request, str, str}, GoFunc: "_ink_setHeader", Chunk: "httpserver"},
		&Spec{Name: "http.respond", Params: []types.Type{Request, types.Int, str}, GoFunc: "_ink_respond", Chunk: "httpserver"},

		// Client.
		&Spec{Name: "http.get", Params: []types.Type{str}, Ret: Response, GoFunc: "_ink_httpGet", Chunk: "httpclient"},
		&Spec{Name: "http.post", Params: []types.Type{str, str}, Ret: Response, GoFunc: "_ink_httpPost", Chunk: "httpclient"},
		&Spec{Name: "http.request", Params: []types.Type{str, str, strList, str}, Ret: Response, GoFunc: "_ink_httpRequest", Chunk: "httpclient"},
		&Spec{Name: "http.status", Params: []types.Type{Response}, Ret: types.Int, GoFunc: "_ink_respStatus", Chunk: "httpclient"},
		&Spec{Name: "http.text", Params: []types.Type{Response}, Ret: str, GoFunc: "_ink_respText", Chunk: "httpclient"},
		&Spec{Name: "http.ok", Params: []types.Type{Response}, Ret: types.Bool, GoFunc: "_ink_respOK", Chunk: "httpclient"},
	)

	// Handle-zero-value note: a handle global read before its declaration
	// executes is nil (SPEC §6.2), and prelude panics happen before any
	// //line directive, so every helper guards nil with a self-describing
	// message instead of letting Go nil-deref in an unmapped frame.
	Chunks["httpserver"] = &Chunk{
		Imports: []string{"io", "net", "net/http", "time"},
		Src: `type _ink_request struct {
	w        http.ResponseWriter
	r        *http.Request
	reqBody  string
	answered bool
	done     chan struct{}
}

func (q *_ink_request) String() string {
	if q == nil {
		return "<nil>"
	}
	return "<request>"
}

type _ink_server struct {
	reqs    chan *_ink_request
	pending *_ink_request // handed out by the last http.next, not yet answered
}

func (s *_ink_server) String() string {
	if s == nil {
		return "<nil>"
	}
	return "<server>"
}

// _ink_serve listens synchronously — a taken port panics here, in the frame
// mapped to the http.serve call — then accepts in the background. Handler
// goroutines park each request on the queue and wait until it is answered,
// so the program handles exactly one request at a time. Server-level
// timeouts guard against slow or idle clients but never fire on a request
// waiting its turn in the queue (the body is already read, and there is no
// per-response write deadline).
func _ink_serve(port int) *_ink_server {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		panic("http.serve: " + err.Error())
	}
	srv := &_ink_server{reqs: make(chan *_ink_request, 64)}
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		IdleTimeout:       2 * time.Minute,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			q := &_ink_request{w: w, r: r, reqBody: string(b), done: make(chan struct{})}
			srv.reqs <- q
			<-q.done
		}),
	}
	go func() { _ = server.Serve(ln) }()
	return srv
}

func _ink_next(s *_ink_server) *_ink_request {
	if s == nil {
		panic("http.next: server handle is uninitialized")
	}
	if s.pending != nil && !s.pending.answered {
		panic("http.next: the previous request was never answered (every request needs exactly one http.respond)")
	}
	q := <-s.reqs
	s.pending = q
	return q
}

func _ink_reqGuard(q *_ink_request, fn string) {
	if q == nil {
		panic(fn + ": request handle is uninitialized")
	}
}

func _ink_method(q *_ink_request) string { _ink_reqGuard(q, "http.method"); return q.r.Method }

func _ink_path(q *_ink_request) string { _ink_reqGuard(q, "http.path"); return q.r.URL.Path }

func _ink_body(q *_ink_request) string { _ink_reqGuard(q, "http.body"); return q.reqBody }

func _ink_header(q *_ink_request, name string) string {
	_ink_reqGuard(q, "http.header")
	return q.r.Header.Get(name)
}

func _ink_query(q *_ink_request, name string) string {
	_ink_reqGuard(q, "http.query")
	return q.r.URL.Query().Get(name)
}

func _ink_setHeader(q *_ink_request, name, value string) {
	_ink_reqGuard(q, "http.set_header")
	if q.answered {
		panic("http.set_header: the request was already answered")
	}
	q.w.Header().Set(name, value)
}

// _ink_respond flushes before releasing the handler goroutine: with an
// explicit Content-Length there is no chunked terminator pending, so the
// response reaches the client even if the program exits immediately after.
func _ink_respond(q *_ink_request, status int, body string) {
	_ink_reqGuard(q, "http.respond")
	if q.answered {
		panic("http.respond: the request was already answered")
	}
	if status < 100 || status > 599 {
		panic("http.respond: invalid status code " + strconv.Itoa(status))
	}
	q.answered = true
	q.w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	q.w.WriteHeader(status)
	_, _ = io.WriteString(q.w, body)
	if f, ok := q.w.(http.Flusher); ok {
		f.Flush()
	}
	close(q.done)
}
`,
	}

	Chunks["httpclient"] = &Chunk{
		Imports: []string{"io", "net/http", "strings", "time"},
		Src: `type _ink_response struct {
	status   int
	respBody string
	ok       bool
}

func (r *_ink_response) String() string {
	if r == nil {
		return "<nil>"
	}
	return "<response>"
}

var _ink_httpClient = &http.Client{Timeout: 10 * time.Minute}

// _ink_httpRequest never panics: transport failures return a response with
// ok=false, status 0, and the error message as the body.
func _ink_httpRequest(method, url string, headers []string, body string) *_ink_response {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return &_ink_response{respBody: err.Error()}
	}
	for _, h := range headers {
		i := strings.Index(h, ":")
		if i <= 0 {
			panic("http.request: header entries must look like \"Name: value\", got " + strconv.Quote(h))
		}
		req.Header.Set(strings.TrimSpace(h[:i]), strings.TrimSpace(h[i+1:]))
	}
	resp, err := _ink_httpClient.Do(req)
	if err != nil {
		return &_ink_response{respBody: err.Error()}
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return &_ink_response{status: resp.StatusCode, respBody: err.Error()}
	}
	return &_ink_response{status: resp.StatusCode, respBody: string(b), ok: true}
}

func _ink_httpGet(url string) *_ink_response { return _ink_httpRequest("GET", url, nil, "") }

func _ink_httpPost(url, body string) *_ink_response {
	return _ink_httpRequest("POST", url, []string{"Content-Type: application/json"}, body)
}

func _ink_respGuard(r *_ink_response, fn string) {
	if r == nil {
		panic(fn + ": response handle is uninitialized")
	}
}

func _ink_respStatus(r *_ink_response) int { _ink_respGuard(r, "http.status"); return r.status }

func _ink_respText(r *_ink_response) string { _ink_respGuard(r, "http.text"); return r.respBody }

func _ink_respOK(r *_ink_response) bool { _ink_respGuard(r, "http.ok"); return r.ok }
`,
	}
}
