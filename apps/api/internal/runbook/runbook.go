package runbook

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("runbook not found")
var ErrInvalid = errors.New("invalid runbook")

type Book struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	ServiceCode string    `json:"serviceCode"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Store struct {
	mu    sync.Mutex
	books map[string]Book
}

func NewStore() *Store {
	return &Store{books: map[string]Book{}}
}

func (s *Store) Seed() {
	_ = s.Upsert(Book{
		ID:          "rb-inventory-5xx",
		Title:       "Inventory 5xx (virtual)",
		Body:        "1. Observe virtual metrics.\n2. Do not kubectl.\n3. For bad-deploy training, rollback is the intended recovery.",
		ServiceCode: "commerce-inventory",
		UpdatedAt:   time.Unix(0, 0).UTC(),
	})
}

func (s *Store) List() []Book {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Book, 0, len(s.books))
	for _, b := range s.books {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) Get(id string) (Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[id]
	if !ok {
		return Book{}, ErrNotFound
	}
	return b, nil
}

func (s *Store) Upsert(b Book) error {
	if b.ID == "" || b.Title == "" || b.Body == "" {
		return ErrInvalid
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books[b.ID] = b
	return nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.books[id]; !ok {
		return ErrNotFound
	}
	delete(s.books, id)
	return nil
}

type OnCall struct {
	At          string `json:"at"`
	Primary     string `json:"primary"`
	Secondary   string `json:"secondary"`
	Note        string `json:"note"`
	VirtualOnly bool   `json:"virtualOnly"`
}

var rotation = [][2]string{
	{"aoki.haru", "sato.mei"},
	{"sato.mei", "kondo.minato"},
	{"kondo.minato", "aoki.haru"},
	{"fujii.an", "sato.mei"},
	{"murakami.hayate", "aoki.haru"},
	{"okada.ritsu", "sato.mei"},
	{"nakamura.nagi", "aoki.haru"},
}

func OnCallAt(at time.Time) OnCall {
	pair := rotation[int(at.UTC().Weekday())%len(rotation)]
	return OnCall{
		At:          at.UTC().Format(time.RFC3339),
		Primary:     pair[0],
		Secondary:   pair[1],
		Note:        "Fictional rotation. Nobody is paged. Learning demo only.",
		VirtualOnly: true,
	}
}

type HistoryEntry struct {
	At       time.Time `json:"at"`
	Scenario string    `json:"scenario"`
	Score    int       `json:"score"`
	Passed   bool      `json:"passed"`
}

type History struct {
	mu      sync.Mutex
	entries []HistoryEntry
}

func NewHistory() *History { return &History{} }

func (h *History) Add(e HistoryEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append([]HistoryEntry{e}, h.entries...)
	if len(h.entries) > 50 {
		h.entries = h.entries[:50]
	}
}

func (h *History) List() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HistoryEntry, len(h.entries))
	copy(out, h.entries)
	return out
}
