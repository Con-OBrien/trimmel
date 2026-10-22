// Package storage answers "what is using my disk?". It measures the folders
// inside a directory so the user can drill down, and finds the largest files
// under a folder. It only reads; nothing here deletes or changes anything.
package storage

import (
	"container/heap"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// scanBudget caps one analysis. A whole system drive can take minutes; after
// the budget the totals measured so far are returned and marked partial.
const scanBudget = 4 * time.Minute

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  uint64 `json:"size"`
	Files int64  `json:"files"`
	IsDir bool   `json:"isDir"`
	Hint  string `json:"hint"` // plain-English note for well-known folders, may be empty
}

type Result struct {
	Path    string  `json:"path"`
	Total   uint64  `json:"total"`
	Entries []Entry `json:"entries"` // largest first
	Partial bool    `json:"partial"` // the time budget ran out
	Skipped int     `json:"skipped"` // folders that could not be read
}

func contextWithBudget(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// isLink reports symlinks and junctions. They are never followed: a link can
// point at another drive or loop back, and would double-count space.
func isLink(t fs.FileMode) bool { return t&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// Analyze measures every entry directly inside dir.
func Analyze(dir string) (Result, error) {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		return Result{}, errors.New("choose a full folder path")
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return Result{}, errors.New("that folder isn't available")
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), scanBudget)
	defer cancel()

	res := Result{Path: dir}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, de := range des {
		if isLink(de.Type()) {
			continue
		}
		e := Entry{Name: de.Name(), Path: filepath.Join(dir, de.Name()), IsDir: de.IsDir()}
		e.Hint = Describe(e.Path)
		if !de.IsDir() {
			if info, err := de.Info(); err == nil {
				e.Size, e.Files = uint64(info.Size()), 1
			}
			res.Entries = append(res.Entries, e)
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(e Entry) {
			defer wg.Done()
			defer func() { <-sem }()
			size, files, skipped := measure(ctx, e.Path)
			e.Size, e.Files = size, files
			mu.Lock()
			res.Entries = append(res.Entries, e)
			res.Skipped += skipped
			mu.Unlock()
		}(e)
	}
	wg.Wait()

	res.Partial = ctx.Err() != nil
	for _, e := range res.Entries {
		res.Total += e.Size
	}
	sort.Slice(res.Entries, func(i, j int) bool { return res.Entries[i].Size > res.Entries[j].Size })
	return res, nil
}

// measure totals the regular files under root.
func measure(ctx context.Context, root string) (size uint64, files int64, skipped int) {
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			skipped++
			return nil
		}
		if isLink(d.Type()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			size += uint64(info.Size())
			files++
		}
		return nil
	})
	return
}

// ---- Largest files ----

type fileHeap []Entry

func (h fileHeap) Len() int            { return len(h) }
func (h fileHeap) Less(i, j int) bool  { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x interface{}) { *h = append(*h, x.(Entry)) }
func (h *fileHeap) Pop() interface{} {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// LargestFiles returns the n biggest files under root, largest first. partial
// is true if the time budget ran out before the whole tree was read.
func LargestFiles(root string, n int) (files []Entry, partial bool, err error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return nil, false, errors.New("choose a full folder path")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, false, errors.New("that folder isn't available")
	}
	if n < 1 {
		n = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), scanBudget)
	defer cancel()

	h := &fileHeap{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if isLink(d.Type()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		sz := uint64(info.Size())
		if h.Len() < n {
			heap.Push(h, Entry{Name: d.Name(), Path: p, Size: sz, Files: 1})
		} else if sz > (*h)[0].Size {
			(*h)[0] = Entry{Name: d.Name(), Path: p, Size: sz, Files: 1}
			heap.Fix(h, 0)
		}
		return nil
	})
	out := make([]Entry, h.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(h).(Entry)
	}
	return out, ctx.Err() != nil, nil
}
