package dakera

import (
	"fmt"
	"sort"
)

// redacted replaces a secret in String / GoString output.
const redacted = "<redacted>"

// String describes the options without the API key, so logging them with
// %v or %+v never prints the secret.
func (o ClientOptions) String() string {
	key := ""
	if o.APIKey != "" {
		key = redacted
	}
	headers := make([]string, 0, len(o.Headers))
	for k := range o.Headers {
		headers = append(headers, k)
	}
	sort.Strings(headers)
	return fmt.Sprintf("ClientOptions{BaseURL:%q APIKey:%q Timeout:%s ConnectTimeout:%s MaxRetries:%d Headers:%v Preflight:%t OdeURL:%q CustomTransport:%t}",
		o.BaseURL, key, o.Timeout, o.ConnectTimeout, o.MaxRetries, headers, o.Preflight, o.OdeURL, o.Transport != nil)
}

// GoString is String (for %#v), so the API key is redacted there too.
// Header values are left out: a custom header may carry a credential.
func (o ClientOptions) GoString() string { return o.String() }

// String describes the client without its API key or header values, so
// logging a *Client with %v, %+v or %#v never prints the secret.
func (c *Client) String() string {
	if c == nil {
		return "dakera.Client(nil)"
	}
	auth := "none"
	if c.apiKey != "" {
		auth = redacted
	}
	return fmt.Sprintf("dakera.Client{baseURL:%q apiKey:%s}", c.baseURL, auth)
}

// GoString is String (for %#v).
func (c *Client) GoString() string { return c.String() }
