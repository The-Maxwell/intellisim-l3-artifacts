package mongodb

import (
	"net/url"
	"strings"
)

// buildURI constructs a MongoDB URI based on the provided configuration.
func buildURI(config map[string]interface{}) string {
	// Default values
	defaults := map[string]interface{}{
		"host": "localhost",
	}

	// Merge defaults with the provided config
	for k, v := range defaults {
		if _, exists := config[k]; !exists {
			config[k] = v
		}
	}

	// Start building the URI
	var uri strings.Builder
	uri.WriteString("mongodb://")

	// Username and password
	if username, ok := config["username"].(string); ok {
		uri.WriteString(url.QueryEscape(username))
		if password, ok := config["password"].(string); ok {
			uri.WriteString(":" + url.QueryEscape(password))
		}
		uri.WriteString("@")
	}

	// Host and potentially first port
	uri.WriteString(config["host"].(string))
	if port, ok := config["port"].(string); ok {
		uri.WriteString(":" + port)
	}

	// Replicas
	if replicas, ok := config["replicas"].([]map[string]interface{}); ok {
		for _, replica := range replicas {
			uri.WriteString("," + replica["host"].(string))
			if port, ok := replica["port"].(string); ok {
				uri.WriteString(":" + port)
			}
		}
	}

	// Database
	if database, ok := config["database"].(string); ok {
		uri.WriteString("/" + database)
	}

	// Options
	if options, ok := config["options"].(map[string]string); ok {
		var pairs []string
		for k, v := range options {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
		if len(pairs) > 0 {
			uri.WriteString("?" + strings.Join(pairs, "&"))
		}
	}

	return uri.String()
}
