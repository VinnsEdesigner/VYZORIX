module github.com/VinnsEdesigner/vyzorix/apps/cli

go 1.27.2

require (
	github.com/VinnsEdesigner/vyzorix/apps/api v0.0.0
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
)

require golang.org/x/text v0.43.0 // indirect

replace github.com/VinnsEdesigner/vyzorix/apps/api => ../api
