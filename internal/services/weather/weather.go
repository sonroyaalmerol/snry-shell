package weather

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

type HTTPClient interface {
	Get(url string) (io.ReadCloser, error)
}

type realHTTPClient struct {
	client *http.Client
}

func (r *realHTTPClient) Get(url string) (io.ReadCloser, error) {
	resp, err := r.client.Get(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func NewHTTPClient() HTTPClient {
	return &realHTTPClient{client: &http.Client{Timeout: 10 * time.Second}}
}

type wttrResponse struct {
	Current []struct {
		TempC          string `json:"temp_C"`
		WeatherDesc    []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
		Humidity       int     `json:"humidity"`
		WindspeedKmph  float64 `json:"windspeedKmph"`
	} `json:"current_condition"`
}

type Service struct {
	bus      *bus.Bus
	client   HTTPClient
	interval time.Duration
}

func New(client HTTPClient, b *bus.Bus) *Service {
	return &Service{bus: b, client: client, interval: 10 * time.Minute}
}

func (s *Service) Run(ctx context.Context) error {
	s.poll()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.poll()
		}
	}
}

func (s *Service) poll() {
	body, err := s.client.Get("https://wttr.in/?format=j1")
	if err != nil {
		return
	}
	defer body.Close()

	var resp wttrResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil || len(resp.Current) == 0 {
		return
	}

	c := resp.Current[0]
	temp, _ := strconv.Atoi(c.TempC)

	condition := ""
	icon := "cloud"
	if len(c.WeatherDesc) > 0 {
		condition = c.WeatherDesc[0].Value
		icon = conditionToIcon(condition)
	}

	s.bus.Publish(bus.TopicWeather, state.WeatherState{
		TempC:     temp,
		Condition: condition,
		Icon:      icon,
		Humidity:  c.Humidity,
		WindKph:   c.WindspeedKmph,
	})
}

func conditionToIcon(cond string) string {
	l := strings.ToLower(cond)
	switch {
	case strings.Contains(l, "sunny") || strings.Contains(l, "clear"):
		return "light_mode"
	case strings.Contains(l, "cloud") || strings.Contains(l, "overcast"):
		return "cloud"
	case strings.Contains(l, "rain") || strings.Contains(l, "drizzle"):
		return "rainy"
	case strings.Contains(l, "snow") || strings.Contains(l, "sleet"):
		return "weather_snowy"
	case strings.Contains(l, "thunder") || strings.Contains(l, "storm"):
		return "thunderstorm"
	case strings.Contains(l, "fog") || strings.Contains(l, "mist"):
		return "foggy"
	default:
		return "cloud"
	}
}
