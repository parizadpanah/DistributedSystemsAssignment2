package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type record struct {
	Collection string          `json:"collection"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
	TS         time.Time       `json:"ts"`
	Tombstone  bool            `json:"tombstone"`
}

type entry struct {
	Value json.RawMessage
}

type collectionState struct {
	mu        sync.RWMutex
	file      *os.File
	writer    *bufio.Writer
	index     map[string]entry
	lines     int
	lastFlush time.Time
}

type store struct {
	dir     string
	mu      sync.RWMutex
	cols    map[string]*collectionState
	flushEv chan struct{}
}

func openStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &store{
		dir:     dir,
		cols:    make(map[string]*collectionState),
		flushEv: make(chan struct{}, 1),
	}

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") {
			coll := strings.TrimSuffix(d.Name(), ".jsonl")
			if _, err := s.openCollection(coll); err != nil {
				log.Printf("open %s: %v", coll, err)
			}
		}
		return nil
	})

	if _, ok := s.cols["default"]; !ok {
		if _, err := s.openCollection("default"); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// -----------------------Collection implementation-----------------------
func (s *store) openCollection(name string) (*collectionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.cols[name]; ok {
		return st, nil
	}
	fp := filepath.Join(s.dir, name+".jsonl")
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	st := &collectionState{
		file:   f,
		writer: bufio.NewWriterSize(f, 256*1024),
		index:  make(map[string]entry),
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 10*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			log.Printf("skip corrupt line in %s: %v", f.Name(), err)
			continue
		}
		if !r.Tombstone {
			st.index[r.Key] = entry{Value: r.Value}
		} else {
			delete(st.index, r.Key)
		}
		st.lines++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	s.cols[name] = st
	return st, nil
}

// -----------------------PUT/objects-----------------------
func (s *store) put(collection, key string, value json.RawMessage) error {
	if strings.TrimSpace(collection) == "" {
		collection = "default"
	}
	st, err := s.openCollection(collection)
	if err != nil {
		return err
	}
	var tmp any
	if err := json.Unmarshal(value, &tmp); err != nil {
		return fmt.Errorf("value must be valid JSON: %w", err)
	}
	// Append JSON-encoded record to the collection file buffer
	rec := record{
		Collection: collection,
		Key:        key,
		Value:      value,
		TS:         time.Now().UTC(),
		Tombstone:  false,
	}
	b, _ := json.Marshal(rec)
	st.mu.Lock()
	defer st.mu.Unlock()
	_, err = st.writer.Write(b)

	if err == nil {
		_, err = st.writer.WriteString("\n")
	}
	if err != nil {
		return err
	}
	st.index[key] = entry{Value: value}
	st.lines++
	if st.writer.Buffered() > 256*1024 || time.Since(st.lastFlush) > 2*time.Second {
		st.writer.Flush()
		st.lastFlush = time.Now()
	}
	if st.lines > 1000 && st.lines > 2*len(st.index) {
		go s.compact(collection)
	}
	return nil
}

// -----------------------GET/objects{key}-----------------------
func (s *store) get(collection, key string) (json.RawMessage, bool) {
	if strings.TrimSpace(collection) == "" {
		collection = "default"
	}
	st, err := s.openCollection(collection)
	if err != nil {
		return nil, false
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	e, ok := st.index[key]
	return e.Value, ok
}

type listItem struct {
	Collection string          `json:"collection,omitempty"`
	Key        string          `json:"key"`
	Value      json.RawMessage `json:"value"`
}

// -----------------------GET/objects-----------------------
func (s *store) list(collection, prefix string, limit, offset int, includeCollection bool, w io.Writer) error {
	//Limitation
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}

	s.mu.RLock()
	var cols []string
	if collection != "" {
		cols = []string{collection}
	} else {
		for c := range s.cols {
			cols = append(cols, c)
		}
		sort.Strings(cols)
	}
	s.mu.RUnlock()

	enc := json.NewEncoder(w)
	_, _ = w.Write([]byte("["))
	wrote := 0
	first := true
	skipped := 0

	for _, c := range cols {
		st, err := s.openCollection(c)
		if err != nil {
			continue
		}
		st.mu.RLock()
		keys := make([]string, 0, len(st.index))
		for k := range st.index {
			//filter by prefix
			if prefix == "" || strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if skipped < offset {
				skipped++
				continue
			}
			if wrote >= limit {
				break
			}
			item := listItem{Key: k, Value: st.index[k].Value}
			if includeCollection {
				item.Collection = c
			}
			if !first {
				_, _ = w.Write([]byte(","))
			} else {
				first = false
			}
			if err := enc.Encode(item); err != nil {
				st.mu.RUnlock()
				return err
			}
			_, _ = w.Write([]byte{})
			wrote++
		}
		st.mu.RUnlock()
		if wrote >= limit {
			break
		}
	}
	_, _ = w.Write([]byte("]"))
	return nil
}

// -----------------------Compact-----------------------
// compresses to keep only the latest version of each key
func (s *store) compact(collection string) {
	st, err := s.openCollection(collection)
	if err != nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	tmpPath := filepath.Join(s.dir, collection+".tmp")
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("compact open tmp: %v", err)
		return
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 256*1024)
	now := time.Now().UTC()
	for k, e := range st.index {
		rec := record{Collection: collection, Key: k, Value: e.Value, TS: now}
		b, _ := json.Marshal(rec)
		w.Write(b)
		w.WriteString("\n")
	}
	w.Flush()

	oldPath := filepath.Join(s.dir, collection+".jsonl")
	if err := os.Rename(tmpPath, oldPath); err != nil {
		log.Printf("compact rename: %v", err)
		return
	}
	st.file.Close()
	newf, err := os.OpenFile(oldPath, os.O_RDWR, 0o644)
	if err != nil {
		log.Printf("compact reopen: %v", err)
		return
	}
	st.file = newf
	st.writer = bufio.NewWriterSize(newf, 256*1024)
	st.lines = len(st.index)
	st.lastFlush = time.Now()
	log.Printf("compacted collection=%s to %d lines", collection, st.lines)
}

// ------------------- MAIM : HTTP Server ----------------------

type apiServer struct {
	st *store
}

func main() {
	addr := getenv("APP_ADDR", ":8080")
	dataDir := getenv("DATA_DIR", "./data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	st, err := openStore(dataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	s := &apiServer{st: st}

	http.HandleFunc("/objects", s.handleObjects) // PUT /objects, GET /objects{list}
	http.HandleFunc("/objects/", s.handleByKey)  // GET /objects/{key}
	log.Printf("listening on %s (data: %s)", addr, dataDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// PUT /objects  body: {"key":"...","value":<json>} ?collection=NAME
func (s *apiServer) handleObjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		s.handlePut(w, r)
	case http.MethodGet:
		s.handleList(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type putReq struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func (s *apiServer) handlePut(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	defer r.Body.Close()
	var req putReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" || strings.Contains(req.Key, "/") {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	collection := strings.TrimSpace(r.URL.Query().Get("collection"))
	if collection == "" {
		collection = "default"
	}
	if err := s.st.put(collection, req.Key, req.Value); err != nil {
		http.Error(w, "store error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// GET /objects/{key}?collection=NAME
func (s *apiServer) handleByKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/objects/")
	if key == "" || strings.Contains(key, "/") {
		http.Error(w, "bad key", http.StatusBadRequest)
		return
	}
	if k, err := url.PathUnescape(key); err == nil {
		key = k
	}
	collection := strings.TrimSpace(r.URL.Query().Get("collection"))
	if collection == "" {
		collection = "default"
	}
	val, ok := s.st.get(collection, key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(val)
}

// GET /objects?limit=?&offset=?&prefix=?&collection=NAME&includeCollection=true
func (s *apiServer) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := atoiInRange(q.Get("limit"), 100, 1, 10000)
	offset := atoiInRange(q.Get("offset"), 0, 0, 1<<31-1)
	collection := strings.TrimSpace(q.Get("collection"))
	prefix := q.Get("prefix")
	includeCollection := strings.EqualFold(q.Get("includeCollection"), "true")

	w.Header().Set("Content-Type", "application/json")
	if err := s.st.list(collection, prefix, limit, offset, includeCollection, w); err != nil {
		http.Error(w, "list error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func atoiInRange(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

var _ = errors.New
