package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// weather.gov REQUIRES a User-Agent identifying your app (contact email recommended).
const userAgent = "my-weather-app/1.0 (you@example.com)"

var client = &http.Client{Timeout: 10 * time.Second}

// --- Structs for the parts of the API responses we care about ---

// Open-Meteo geocoding response: https://open-meteo.com/en/docs/geocoding-api
type geocodeResponse struct {
	Results []struct {
		Name       string  `json:"name"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		Admin1     string  `json:"admin1"` // state, e.g. "Colorado"
		Population int     `json:"population"`
	} `json:"results"`
}

type pointsResponse struct {
	Properties struct {
		Forecast string `json:"forecast"`
	} `json:"properties"`
}

type Period struct {
	Name             string `json:"name"`
	Temperature      int    `json:"temperature"`
	TemperatureUnit  string `json:"temperatureUnit"`
	WindSpeed        string `json:"windSpeed"`
	WindDirection    string `json:"windDirection"`
	ShortForecast    string `json:"shortForecast"`
	DetailedForecast string `json:"detailedForecast"`
}

type forecastResponse struct {
	Properties struct {
		Periods []Period `json:"periods"`
	} `json:"properties"`
}

type Forecast struct {
	Location string   `json:"location"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	Periods  []Period `json:"periods"`
}

// getJSON performs a GET with the required headers and decodes the result.
func getJSON(u string, out any) error {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/geo+json, application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s", req.URL.Host, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// --- US states (weather.gov covers the 50 states, DC and territories) ---

type State struct{ Code, Name string }

var states = []State{
	{"AL", "Alabama"}, {"AK", "Alaska"}, {"AZ", "Arizona"}, {"AR", "Arkansas"},
	{"CA", "California"}, {"CO", "Colorado"}, {"CT", "Connecticut"}, {"DE", "Delaware"},
	{"DC", "District of Columbia"}, {"FL", "Florida"}, {"GA", "Georgia"}, {"HI", "Hawaii"},
	{"ID", "Idaho"}, {"IL", "Illinois"}, {"IN", "Indiana"}, {"IA", "Iowa"},
	{"KS", "Kansas"}, {"KY", "Kentucky"}, {"LA", "Louisiana"}, {"ME", "Maine"},
	{"MD", "Maryland"}, {"MA", "Massachusetts"}, {"MI", "Michigan"}, {"MN", "Minnesota"},
	{"MS", "Mississippi"}, {"MO", "Missouri"}, {"MT", "Montana"}, {"NE", "Nebraska"},
	{"NV", "Nevada"}, {"NH", "New Hampshire"}, {"NJ", "New Jersey"}, {"NM", "New Mexico"},
	{"NY", "New York"}, {"NC", "North Carolina"}, {"ND", "North Dakota"}, {"OH", "Ohio"},
	{"OK", "Oklahoma"}, {"OR", "Oregon"}, {"PA", "Pennsylvania"}, {"PR", "Puerto Rico"},
	{"RI", "Rhode Island"}, {"SC", "South Carolina"}, {"SD", "South Dakota"}, {"TN", "Tennessee"},
	{"TX", "Texas"}, {"UT", "Utah"}, {"VT", "Vermont"}, {"VA", "Virginia"},
	{"WA", "Washington"}, {"WV", "West Virginia"}, {"WI", "Wisconsin"}, {"WY", "Wyoming"},
}

// stateName accepts "CO", "co" or "Colorado" and returns the full name ("" if unknown).
func stateName(s string) string {
	s = strings.TrimSpace(s)
	for _, st := range states {
		if strings.EqualFold(s, st.Code) || strings.EqualFold(s, st.Name) {
			return st.Name
		}
	}
	return ""
}

// --- Geocoding (city + state -> lat/lon) via Open-Meteo (free, no API key) ---

type coords struct {
	Lat, Lon float64
	Name     string
}

// Cache lookups so repeat searches don't call the geocoder again.
var (
	geoCache   = map[string]coords{}
	geoCacheMu sync.Mutex
)

func geocodeCity(city, state string) (coords, error) {
	city = strings.TrimSpace(city)
	fullState := stateName(state)
	if city == "" || fullState == "" {
		return coords{}, fmt.Errorf("please enter a city and pick a valid US state")
	}
	key := strings.ToLower(city + "|" + fullState)

	geoCacheMu.Lock()
	if c, ok := geoCache[key]; ok {
		geoCacheMu.Unlock()
		return c, nil
	}
	geoCacheMu.Unlock()

	// Search US places by name; we then pick the first result in the chosen state.
	q := url.Values{}
	q.Set("name", city)
	q.Set("countryCode", "US") // weather.gov only covers the US
	q.Set("count", "50")       // enough to cover every "Springfield" in the country
	q.Set("language", "en")
	q.Set("format", "json")

	var g geocodeResponse
	if err := getJSON("https://geocoding-api.open-meteo.com/v1/search?"+q.Encode(), &g); err != nil {
		return coords{}, err
	}

	// Results come back ranked by relevance/population, so the first state match is best.
	for _, r := range g.Results {
		if strings.EqualFold(r.Admin1, fullState) {
			c := coords{
				Lat:  r.Latitude,
				Lon:  r.Longitude,
				Name: r.Name + ", " + r.Admin1,
			}
			geoCacheMu.Lock()
			geoCache[key] = c
			geoCacheMu.Unlock()
			return c, nil
		}
	}
	return coords{}, fmt.Errorf("couldn't find %q in %s", city, fullState)
}

// --- Weather (lat/lon -> forecast) via api.weather.gov ---

func fetchForecast(city, state string) (*Forecast, error) {
	c, err := geocodeCity(city, state)
	if err != nil {
		return nil, err
	}

	// Step 1: /points gives us the forecast URL for this grid square (max 4 decimals).
	var p pointsResponse
	if err := getJSON(fmt.Sprintf("https://api.weather.gov/points/%.4f,%.4f", c.Lat, c.Lon), &p); err != nil {
		return nil, err
	}
	if p.Properties.Forecast == "" {
		return nil, fmt.Errorf("no forecast available for this location")
	}

	// Step 2: fetch the forecast periods.
	var f forecastResponse
	if err := getJSON(p.Properties.Forecast, &f); err != nil {
		return nil, err
	}

	return &Forecast{Location: c.Name, Lat: c.Lat, Lon: c.Lon, Periods: f.Properties.Periods}, nil
}

// --- Handlers ---

// GET /api/forecast?city=Seattle&state=WA  -> JSON  (state can be "WA" or "Washington")
func apiForecastHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	city := r.URL.Query().Get("city")
	state := r.URL.Query().Get("state")

	fc, err := fetchForecast(city, state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(fc)
}

var page = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Weather</title>
<style>body{font-family:sans-serif;max-width:700px;margin:2rem auto}
.p{border-bottom:1px solid #ddd;padding:.5rem 0}.err{color:#b00}
input,select,button{font-size:1rem;padding:.3rem}</style></head>
<body>
<h1>US City 7 Day Forecast</h1>
<form method="GET" action="/">
  <input name="city" value="{{.City}}" placeholder="City" required autofocus>
  <select name="state" required>
    <option value="">State</option>
    {{range .States}}<option value="{{.Code}}"{{if eq .Code $.State}} selected{{end}}>{{.Name}}</option>
    {{end}}
  </select>
  <button>Get forecast</button>
</form>
{{if .Err}}<p class="err">{{.Err}}</p>{{end}}
{{with .Forecast}}
  <h2>{{.Location}}</h2>
  {{range .Periods}}
    <div class="p"><b>{{.Name}}</b>: {{.Temperature}}°{{.TemperatureUnit}}, {{.ShortForecast}}
    <br><small>Wind {{.WindSpeed}} {{.WindDirection}}</small></div>
  {{end}}
{{end}}
</body></html>`))

// GET / -> HTML page with city input + state dropdown
func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		City, State string
		States      []State
		Forecast    *Forecast
		Err         string
	}{
		City:   strings.TrimSpace(r.URL.Query().Get("city")),
		State:  strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("state"))),
		States: states,
	}

	if data.City != "" || data.State != "" {
		if fc, err := fetchForecast(data.City, data.State); err != nil {
			data.Err = err.Error()
		} else {
			data.Forecast = fc
		}
	}

	page.Execute(w, data)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/api/forecast", apiForecastHandler)

	log.Println("Listening on http://localhost:5000")
	log.Fatal(http.ListenAndServe(":5000", mux))
}
