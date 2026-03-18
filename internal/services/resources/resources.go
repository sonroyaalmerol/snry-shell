package resources

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// FileReader reads a file's contents. Used for testability.
type FileReader interface {
	ReadFile(path string) (string, error)
}

type osFileReader struct{}

func (osFileReader) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func NewFileReader() FileReader { return osFileReader{} }

type Service struct {
	bus      *bus.Bus
	reader   FileReader
	interval time.Duration
	prevIdle  uint64
	prevTotal uint64
}

func New(reader FileReader, b *bus.Bus) *Service {
	return &Service{bus: b, reader: reader, interval: 2 * time.Second}
}

func (s *Service) Run(ctx context.Context) error {
	// Initialize CPU baseline from first read.
	s.readCPU()
	s.publish()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.publish()
		}
	}
}

func (s *Service) publish() {
	cpu := s.readCPU()
	ram := s.readRAM()
	s.bus.Publish(bus.TopicResources, state.ResourceState{
		CPU: cpu,
		RAM: ram,
	})
}

func (s *Service) readCPU() float64 {
	data, err := s.reader.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0
		}
		var idle, total uint64
		for i, f := range fields[1:] {
			v, _ := strconv.ParseUint(f, 10, 64)
			total += v
			if i == 3 { // idle is the 4th value (user nice system idle)
				idle = v
			}
		}
		dIdle := idle - s.prevIdle
		dTotal := total - s.prevTotal
		s.prevIdle = idle
		s.prevTotal = total
		if dTotal == 0 {
			return 0
		}
		return float64(dTotal-dIdle) / float64(dTotal) * 100
	}
	return 0
}

func (s *Service) readRAM() float64 {
	data, err := s.reader.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	sc := bufio.NewScanner(strings.NewReader(data))
	var avail, total float64
	for sc.Scan() {
		line := sc.Text()
		v, ok := parseMeminfoLine(line, "MemAvailable")
		if ok {
			avail = v
		}
		v, ok = parseMeminfoLine(line, "MemTotal")
		if ok {
			total = v
		}
	}
	if total == 0 {
		return 0
	}
	return (1 - avail/total) * 100
}

func parseMeminfoLine(line, key string) (float64, bool) {
	prefix := key + ":"
	if !strings.HasPrefix(line, prefix) {
		return 0, false
	}
	val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	val = strings.TrimSuffix(val, " kB")
	n, err := strconv.ParseFloat(val, 64)
	return n, err == nil
}
