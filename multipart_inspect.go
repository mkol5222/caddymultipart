package caddymultipart

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Middleware{})
	httpcaddyfile.RegisterHandlerDirective("multipart_inspector", parseCaddyfile)
}

// Middleware implements an HTTP handler that writes the
// visitor's IP address to a file or stream.
type Middleware struct {
	// The file or stream to write to. Can be "stdout"
	// or "stderr".
	Output string `json:"output,omitempty"`

	w io.Writer
}

// CaddyModule returns the Caddy module information.
func (Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.multipart_inspector",
		New: func() caddy.Module { return new(Middleware) },
	}
}

// Provision implements caddy.Provisioner.
func (m *Middleware) Provision(ctx caddy.Context) error {
	switch m.Output {
	case "stdout":
		m.w = os.Stdout
	case "stderr":
		m.w = os.Stderr
	default:
		return fmt.Errorf("an output stream is required")
	}
	return nil
}

// Validate implements caddy.Validator.
func (m *Middleware) Validate() error {
	if m.w == nil {
		return fmt.Errorf("no writer")
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	m.w.Write([]byte(r.RemoteAddr))
	fmt.Fprintln(m.w, ".")

	if r.Method == http.MethodPost && isMultipart(r) {
		fileNames, err := m.inspectMultipartForm(r)
		if err != nil {
			// m.Logger.Printf("Error inspecting multipart form: %v", err)

			fmt.Fprintf(m.w, "Error inspecting multipart form: %v\n", err)

			// Important:  You might want to return an error to the client here,
			// or continue with the request after logging the error.  It depends on
			// your desired behavior. For now, we'll just log it and continue.
		} else {
			// m.Logger.Printf("Uploaded files: %v", fileNames)
			fmt.Fprintf(m.w, "Uploaded files: %v\n", fileNames)
		}
	}

	return next.ServeHTTP(w, r)
}

// inspectMultipartForm extracts filenames from a multipart form.
func (m *Middleware) inspectMultipartForm(r *http.Request) ([]string, error) {

	// work on copy of request
	r2 := r.Clone(r.Context())
	// close
	defer r2.Body.Close()

	err := r2.ParseMultipartForm(99 << 20) // 32MB is a reasonable default. Adjust as needed.
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

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	// require an argument
	if !d.NextArg() {
		return d.ArgErr()
	}

	// store the argument
	m.Output = d.Val()
	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m Middleware
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddy.Validator             = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
)

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
