# Golang-local-weather

This Go-based local weather application delivers fast, reliable forecasts from the National Weather Service at weather.gov. It can run on both personal computers and production servers. The code is stored and managed in a Github repo and can be shared and edited as needed based on approval. This application will allow user to check the weather in their city or city you might travel to. 

**The code can run on multiple servers in a cluster to allow scalability based on usage. 

**Doing spikes, they’ll be nodes added to the cluster during peak time and spun down when request slow. 

**The application will be monitored using Grafana and Prometheus, both the application layer and host layer will be monitored with an acceptable SLO. 

**To secure the application the code will be stored in Github with access least privilege (PolP) rules applied. 

**The nodes will be scanned and hardened to ensure industrial compliance. 

**If external services are misbehaving, there will be an alert sent to the admins and a return message to the users as we work to resolve the technical issue. 

**The application will be deployed via Github to ensure the code is built properly and current on each node. 



More details about the Golang code built to display accurate weather:
# Go Weather Server

A small Go web server that looks up the National Weather Service forecast for any US city. Enter a city and state in the browser, or call the JSON endpoint.

Uses only the Go standard library — no dependencies.

## Requirements

- Go 1.18 or newer (the cache uses generics)
- Internet access to:
  - `geocoding-api.open-meteo.com` (city → coordinates)
  - `api.weather.gov` (coordinates → forecast)

## Setup

1. Open `main.go` and set `userAgent` to your own app name and contact email. weather.gov rejects requests without a proper User-Agent (403 Forbidden).

   ```go
   const userAgent = "my-weather-app/1.0 (you@example.com)"
   ```

2. Run it:

   ```bash
   go mod init weatherapp
   go run main.go
   ```

3. Open http://localhost:5000

## Usage

### Web page

Go to http://localhost:5000, type a city, pick a state, and click **Get forecast**.

### JSON API

```
GET /api/forecast?city=<city>&state=<state>
```

`state` accepts an abbreviation or full name, case-insensitive (`CO`, `co`, `Colorado`).

Example:

```bash
curl "http://localhost:5000/api/forecast?city=Springfield&state=IL"
```

Response:

```json
{
  "location": "Springfield, Illinois",
  "lat": 39.80172,
  "lon": -89.64371,
  "periods": [
    {
      "name": "Tonight",
      "temperature": 52,
      "temperatureUnit": "F",
      "windSpeed": "5 mph",
      "windDirection": "S",
      "shortForecast": "Mostly Clear",
      "detailedForecast": "Mostly clear, with a low around 52..."
    }
  ]
}
```

Status codes:

| Code | Meaning |
|------|---------|
| 200  | Forecast returned |
| 400  | Missing/invalid city or state, or city not found in that state |
| 502  | Geocoder or weather.gov request failed |

## How it works

1. **Geocode** — the city name is sent to Open-Meteo's geocoding API (US results only). The first result in the chosen state is used.
2. **Points lookup** — `api.weather.gov/points/{lat},{lon}` returns the forecast URL for that NWS grid square.
3. **Forecast** — that URL returns the forecast periods (e.g. "Tonight", "Wednesday").

## Caching

All caches are in memory and reset when the server restarts.

| Cache | Key | Lifetime |
|-------|-----|----------|
| Coordinates | city + state | Forever |
| Forecast URL | lat,lon | Forever |
| Forecast periods | forecast URL | 10 minutes |

A repeat search within 10 minutes makes no outbound requests.

## Concurrency

- Each connection is handled in its own goroutine (standard `net/http` behavior).
- Caches are protected by a mutex, and the shared `http.Client` reuses connections.
- Outbound requests use the incoming request's context, so they're cancelled if the user disconnects.
- Server timeouts: read 10s, write 40s, idle 60s. Outbound requests time out after 10s.

## Troubleshooting

- **403 Forbidden from api.weather.gov** — set a real `userAgent` in `main.go`.
- **"couldn't find ... in ..."** — check the spelling; try the official city name (e.g. "Saint Louis" vs "St. Louis").
- **500/503 from api.weather.gov** — the NWS API has occasional outages; retry after a moment.
- **No forecast for a location** — weather.gov only covers the US and its territories.
