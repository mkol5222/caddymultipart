package caddymultipart

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

// MultipartInspector is a Caddy module that inspects multipart POST requests.
type MultipartInspector struct {
	Next    reverseproxy.RoundTripper `json:"next,omitempty"`     // Required: Next handler in the chain
	LogFile string                    `json:"log_file,omitempty"` //Optional: Log file to use instead of standard error
	Logger  *log.Logger
}

// ServeHTTP implements the caddyhttp.MiddlewareHandler interface.
func (m *MultipartInspector) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	// Only process POST requests with multipart form data.
	if r.Method == http.MethodPost && isMultipart(r) {
		fileNames, err := m.inspectMultipartForm(r)
		if err != nil {
			m.Logger.Printf("Error inspecting multipart form: %v", err)
			// Important:  You might want to return an error to the client here,
			// or continue with the request after logging the error.  It depends on
			// your desired behavior. For now, we'll just log it and continue.
		} else {
			m.Logger.Printf("Uploaded files: %v", fileNames)
		}
	}

	// Call the next handler in the chain.  This is *essential* for the reverse proxy to function.
	if m.Next != nil {
		return m.Next.RoundTrip(w, r)
	} else {
		//This should probably be an error
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Missing next reverse proxy handler"))
		return nil
	}
}

// inspectMultipartForm extracts filenames from a multipart form.
func (m *MultipartInspector) inspectMultipartForm(r *http.Request) ([]string, error) {
	err := r.ParseMultipartForm(32 << 20) // 32MB is a reasonable default. Adjust as needed.
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	var fileNames []string
	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			fileNames = append(fileNames, header.Filename)
		}
	}

	return fileNames, nil
}

// isMultipart checks if the request is a multipart form.
func isMultipart(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	return mediaType == "multipart/form-data"
}

// Interface guards
var (
	_ caddyhttp.MiddlewareHandler = (*MultipartInspector)(nil)
	_ caddy.Provisioner           = (*MultipartInspector)(nil)
)

// Provision implements caddy.Provisioner.
func (m *MultipartInspector) Provision(ctx caddy.Context) error {
	if m.LogFile != "" {
		logFile, err := os.OpenFile(m.LogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		m.Logger = log.New(logFile, "multipart_inspector: ", log.LstdFlags)
	} else {
		m.Logger = log.New(os.Stderr, "multipart_inspector: ", log.LstdFlags)
	}
	return nil
}

// Validate implements caddy.Validator.
func (m *MultipartInspector) Validate() error {
	// You could add validation logic here, such as checking if the log file is writeable, etc.
	return nil
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
// Syntax:
//
//	multipart_inspector {
//		log_file <path>
//	}
func (m *MultipartInspector) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "log_file":
				if !d.NextArg() {
					return d.ArgErr()
				}
				m.LogFile = d.Val()
				if d.NextArg() {
					return d.ArgErr()
				}
			default:
				return d.Errf("unknown subdirective '%s'", d.Val())
			}
		}
	}
	return nil
}

// init registers the module.
func init() {
	caddy.RegisterModule(MultipartInspector{})
}

// Caddyfile Adapter - Required
type caddyfile struct{}

func (c caddyfile) Adapt(d *caddyfile.Dispenser, _ map[string]interface{}) (map[string]interface{}, error) {
	enc := map[string]interface{}{
		"handler": "multipart_inspector",
	}

	mi := &MultipartInspector{}

	err := mi.UnmarshalCaddyfile(d)
	if err != nil {
		return nil, err
	}

	enc["log_file"] = mi.LogFile

	return enc, nil
}
