package feeds

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
)

// SeenStore is a persistent, concurrency-safe store of seen article link hashes.
// It prevents the bot from posting the same article twice across restarts.
type SeenStore struct {
	mu   sync.Mutex
	path string
	seen map[string]bool
}

// NewSeenStore loads (or creates) a SeenStore backed by the given JSON file.
func NewSeenStore(path string) *SeenStore {
	s := &SeenStore{path: path, seen: make(map[string]bool)}
	s.load()
	return s
}

func (s *SeenStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return
	}
	for _, k := range keys {
		s.seen[k] = true
	}
}

func (s *SeenStore) save() {
	keys := make([]string, 0, len(s.seen))
	for k := range s.seen {
		keys = append(keys, k)
	}
	// Cap at 10k entries to prevent unbounded growth
	if len(keys) > 10000 {
		sort.Strings(keys)
		keys = keys[len(keys)-10000:]
	}
	data, _ := json.Marshal(keys)
	os.WriteFile(s.path, data, 0644) //nolint
}

// Has returns true if the link has been seen before.
func (s *SeenStore) Has(link string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[articleHash(link)]
}

// Add marks a link as seen and persists to disk.
func (s *SeenStore) Add(link string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen[articleHash(link)] = true
	s.save()
}
