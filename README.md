# geo-energy-datadog

A Go application that periodically queries the [Geotogether](https://geotogether.com/) smart meter API and writes data to Datadog. It's written in such a way that it ought to be possible to plug in other inputs and outputs fairly easily.

![](./assets/screenshot.png)


## Getting started

```sh
export GEO_USERNAME="me@example.com"
export GEO_PASSWORD="<geotogether.com password here>"
export DD_API_KEY="<Datadog API key here>"
export DD_APP_KEY="<Datadog app key here>" # optional
export DD_SITE="datadoghq.eu" # optional, defaults to datadoghq.com

go run .
```

## Using Docker

Export the environment variables as above, then:

```sh
docker compose up
```
