package service

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	"google.golang.org/protobuf/proto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type benchmarkSnapshot struct {
	data    *hubv1.GetBenchmarkLeaderboardResponse
	updated time.Time
	partial bool
}
type benchmarkSnapshots struct {
	mu      sync.Mutex
	items   map[uint32]benchmarkSnapshot
	pending map[uint32]bool
	queue   chan uint32
}

func (s *HubServer) GetBenchmarkLeaderboard(ctx context.Context, req *connect.Request[hubv1.GetBenchmarkLeaderboardRequest]) (*connect.Response[hubv1.GetBenchmarkLeaderboardResponse], error) {
	id := req.Msg.GetBenchmarkId()
	if id == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("benchmark_id is required"))
	}
	c := &s.snapshots
	c.mu.Lock()
	if c.queue == nil {
		c.items = make(map[uint32]benchmarkSnapshot)
		c.pending = make(map[uint32]bool)
		c.queue = make(chan uint32, 32)
		go s.runBenchmarkSnapshots()
	}
	snapshot, found := c.items[id]
	if (!found || time.Since(snapshot.updated) > 15*time.Minute) && !c.pending[id] {
		select {
		case c.queue <- id:
			c.pending[id] = true
		default:
		}
	}
	c.mu.Unlock()
	if !found {
		response := connect.NewResponse(&hubv1.GetBenchmarkLeaderboardResponse{BenchmarkId: id})
		response.Header().Set("X-Snapshot-Partial", "true")
		response.Header().Set("X-Snapshot-Pending", "true")
		return response, nil
	}
	return pageBenchmarkSnapshot(snapshot, req), nil
}
func pageBenchmarkSnapshot(snapshot benchmarkSnapshot, req *connect.Request[hubv1.GetBenchmarkLeaderboardRequest]) *connect.Response[hubv1.GetBenchmarkLeaderboardResponse] {
	data := proto.Clone(snapshot.data).(*hubv1.GetBenchmarkLeaderboardResponse)
	query, _ := url.PathUnescape(req.Header().Get("X-Page-Query"))
	query = strings.ToLower(strings.TrimSpace(query))
	entries := data.Entries[:0]
	for _, entry := range data.Entries {
		if query == "" || strings.Contains(strings.ToLower(entry.UserHandle+" "+entry.DisplayName), query) {
			entries = append(entries, entry)
		}
	}
	total := len(entries)
	page, _ := strconv.Atoi(req.Header().Get("X-Page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(req.Header().Get("X-Page-Size"))
	if size < 10 || size > 100 {
		size = 50
	}
	page = min(page, max(1, (total+size-1)/size))
	start := min((page-1)*size, total)
	data.Entries = entries[start:min(start+size, total)]
	response := connect.NewResponse(data)
	response.Header().Set("X-Snapshot-Updated", snapshot.updated.UTC().Format(time.RFC3339))
	response.Header().Set("X-Snapshot-Partial", strconv.FormatBool(snapshot.partial))
	response.Header().Set("X-Total-Count", strconv.Itoa(total))
	response.Header().Set("X-Page", strconv.Itoa(page))
	return response
}
func (s *HubServer) runBenchmarkSnapshots() {
	c := &s.snapshots
	for id := range c.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result, err := s.rebuildBenchmarkLeaderboard(kovaaksbenchmarks.FreshResults(ctx), connect.NewRequest(&hubv1.GetBenchmarkLeaderboardRequest{BenchmarkId: id}))
		cancel()
		c.mu.Lock()
		previous := c.items[id]
		if err == nil {
			partial := result.Header().Get("X-Snapshot-Partial") == "true"
			if partial && previous.data != nil {
				seen := map[string]bool{}
				for _, e := range result.Msg.Entries {
					seen[e.UserHandle] = true
				}
				for _, e := range previous.data.Entries {
					if !seen[e.UserHandle] {
						result.Msg.Entries = append(result.Msg.Entries, e)
					}
				}
				sortBenchmarkLeaderboard(result.Msg.Entries)
			}
			if len(c.items) >= 64 {
				for key := range c.items {
					if key != id {
						delete(c.items, key)
						break
					}
				}
			}
			c.items[id] = benchmarkSnapshot{data: result.Msg, updated: time.Now(), partial: partial}
		} else if previous.data != nil {
			previous.partial = true
			c.items[id] = previous
		}
		delete(c.pending, id)
		c.mu.Unlock()
	}
}
