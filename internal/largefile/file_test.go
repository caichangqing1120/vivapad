package largefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture(t testing.TB, chunks ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	for _, chunk := range chunks {
		if _, err := file.WriteString(chunk); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func indexed(t testing.TB, file *File) Progress {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		progress := file.Progress()
		if progress.Err != nil {
			t.Fatal(progress.Err)
		}
		if progress.Done {
			return progress
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("index did not finish")
	return Progress{}
}

func TestMixedEndingsAcrossPagesAndAppend(t *testing.T) {
	prefix := strings.Repeat("x", pageSize-1)
	path := fixture(t, prefix, "\r\nsecond\rthird\nfourth\r")
	file, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	first, err := file.Window(1, 1, 0, 0)
	if err != nil || len(first) != 1 || !first[0].Clipped {
		t.Fatalf("first window: %v %v", len(first), err)
	}
	if got := indexed(t, file).Lines; got != 5 {
		t.Fatalf("lines = %d", got)
	}
	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("\nlast"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if changed, err := file.Refresh(); !changed || err != nil {
		t.Fatalf("refresh = %v, %v", changed, err)
	}
	if got := indexed(t, file).Lines; got != 5 {
		t.Fatalf("append split CRLF: %d lines", got)
	}
	lines, err := file.Window(2, 4, 0, 0)
	if err != nil || len(lines) != 4 || lines[0].Text != "second" || lines[0].Offset != pageSize+1 || lines[3].Text != "last" || lines[3].Offset != int64(len(prefix+"\r\nsecond\rthird\nfourth\r\n")) {
		t.Fatalf("window = %+v, %v", lines, err)
	}
}

func TestStreamingFindAndCancellation(t *testing.T) {
	path := fixture(t, "café ERROR and terror\r\nerror 42\r", strings.Repeat("x", 2<<20), " ERROR 99\n")
	file, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result, err := file.Search(context.Background(), "error", SearchOptions{WholeWord: true}, 2000)
	if err != nil || len(result.Matches) != 3 || result.Matches[0].Column != 6 || result.Matches[1].Offset != int64(len("café ERROR and terror\r\n")) || result.Matches[2].Line != 3 || result.Matches[2].Column != (2<<20)+2 {
		t.Fatalf("literal = %+v, %v", result, err)
	}
	result, err = file.Search(context.Background(), `ERROR \d+$`, SearchOptions{Regex: true}, 1)
	if err != nil || len(result.Matches) != 1 || result.Matches[0].Line != 2 || !result.Truncated {
		t.Fatalf("regex = %+v, %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := file.scanMatches(ctx, "error", SearchOptions{}, func(Match) bool { cancel(); return true }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
}

func TestConcurrentCloseReleasesHandle(t *testing.T) {
	path := fixture(t, strings.Repeat("line without match\n", 400000))
	file, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 6 {
		workers.Go(func() { _, _ = file.Search(context.Background(), "not found", SearchOptions{}, 2000) })
		workers.Go(func() { _, _ = file.Window(1, 100, 0, 0) })
		workers.Go(func() { _ = file.Close() })
	}
	workers.Wait()
	if _, err := file.handle.Stat(); err == nil || (!errors.Is(err, os.ErrClosed) && !strings.Contains(err.Error(), "The handle is invalid")) {
		t.Fatalf("file handle still open: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWithActiveWriterAndFindPastResultLimit(t *testing.T) {
	path := fixture(t, strings.Repeat("hit\n", MaxMatches+5))
	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	file, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := writer.WriteString("hit\n"); err != nil {
		t.Fatal(err)
	}
	if changed, err := file.Refresh(); !changed || err != nil {
		t.Fatalf("refresh = %v, %v", changed, err)
	}
	result, err := file.Search(context.Background(), "hit", SearchOptions{}, MaxMatches+10)
	if err != nil || len(result.Matches) != MaxMatches || !result.Truncated {
		t.Fatalf("count=%d capped=%v err=%v", len(result.Matches), result.Truncated, err)
	}
	hit, found, err := file.Find(context.Background(), "hit", SearchOptions{}, (MaxMatches+4)*4, false)
	if err != nil || !found || hit.Line != MaxMatches+5 {
		t.Fatalf("next after cap: %+v, %v, %v", hit, found, err)
	}
	hit, found, err = file.Find(context.Background(), "hit", SearchOptions{}, 0, true)
	if err != nil || !found || hit.Line != MaxMatches+6 {
		t.Fatalf("wrap to last: %+v, %v, %v", hit, found, err)
	}
}

func largeFixture(t testing.TB) string {
	t.Helper()
	line := strings.Repeat("a", 125) + "\r\n\n"
	block := strings.Repeat(line, 8192)
	chunks := make([]string, 200)
	for index := range chunks {
		chunks[index] = block
	}
	return fixture(t, chunks...)
}

func Test200MiBIndexAndCacheBound(t *testing.T) {
	file, err := Open(largeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	progress := indexed(t, file)
	if progress.Size != 200<<20 || progress.Lines != 2*200*8192+1 {
		t.Fatalf("progress = %+v", progress)
	}
	for line := 1; line < progress.Lines; line += 16384 {
		window, err := file.Window(line, 2, 0, 0)
		if err != nil || len(window) != 2 || window[0].Offset != int64((line-1)/2)*128 || window[1].Text != "" {
			t.Fatalf("line %d: %+v %v", line, window, err)
		}
	}
	if cache := file.CacheBytes(); cache == 0 || cache >= 32<<20 {
		t.Fatalf("cache = %d bytes", cache)
	}
}

func TestSparseFileBeyond1GiB(t *testing.T) {
	if testing.Short() {
		t.Skip("scans a logical 1 GiB sparse file")
	}
	path := fixture(t, "first\n")
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	const offset = (1 << 30) + 7
	const tail = "\r\nlast"
	_, writeErr := writer.WriteAt([]byte(tail), offset)
	if err := errors.Join(writeErr, writer.Close()); err != nil {
		t.Fatal(err)
	}
	file, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if progress := indexed(t, file); progress.Size != offset+int64(len(tail)) || progress.Lines != 3 {
		t.Fatalf("large offset progress: %+v", progress)
	}
	lines, err := file.Window(3, 1, 0, 0)
	if err != nil || len(lines) != 1 || lines[0].Offset != offset+2 || lines[0].Text != "last" {
		t.Fatalf("large offset read: %+v, %v", lines, err)
	}
}

func BenchmarkFirstWindow200MiB(bench *testing.B) {
	path := largeFixture(bench)
	bench.ResetTimer()
	for bench.Loop() {
		file, err := Open(path)
		if err != nil {
			bench.Fatal(err)
		}
		lines, err := file.Window(1, 300, 0, 0)
		bench.StopTimer()
		if err != nil || len(lines) != 300 {
			bench.Fatalf("first screen: %d lines, %v", len(lines), err)
		}
		_ = file.Close()
		bench.StartTimer()
	}
}

func BenchmarkFullIndexHeap200MiB(bench *testing.B) {
	path := largeFixture(bench)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	peak := uint64(0)
	sample := func() {
		runtime.ReadMemStats(&after)
		if after.HeapAlloc > before.HeapAlloc {
			peak = max(peak, after.HeapAlloc-before.HeapAlloc)
		}
	}
	bench.ResetTimer()
	for bench.Loop() {
		file, err := Open(path)
		if err != nil {
			bench.Fatal(err)
		}
		progress := indexed(bench, file)
		sample()
		for line := 1; line < progress.Lines; line += 16384 {
			if _, err := file.Window(line, 300, 0, 0); err != nil {
				bench.Fatal(err)
			}
			sample()
		}
		bench.ReportMetric(float64(file.CacheBytes())/(1<<20), "cache-MiB")
		_ = file.Close()
	}
	bench.ReportMetric(float64(peak)/(1<<20), "peak-sampled-heap-MiB")
	if peak >= 40<<20 {
		bench.Fatalf("sampled additional heap = %.2f MiB", float64(peak)/(1<<20))
	}
}
