package wrapper

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

const (
	contextKey = "context"
)

// WrapFiberHTTPMiddleware wraps net/http middleware to fiber middleware
func WrapFiberHTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var next bool
		netHTTPHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next = true
			c.Request().Header.SetMethod(r.Method)
			c.Request().SetRequestURI(r.RequestURI)
			c.Request().SetHost(r.Host)
			c.Locals(contextKey, r.Context())
			for key, val := range r.Header {
				for _, v := range val {
					c.Request().Header.Set(key, v)
				}
			}
		})
		fiberHandler := func(ctx context.Context, h http.Handler) fiber.Handler {
			return func(c *fiber.Ctx) error {
				WrapNetHTTPToFastHTTPHandler(ctx, h)(c.Context())
				return nil
			}
		}

		fiberHandler(FastHTTPParseContext(c.Context()), mw(netHTTPHandler))(c)
		if next {
			return c.Next()
		}
		return nil
	}
}

// WrapNetHTTPToFastHTTPHandler custom from https://github.com/valyala/fasthttp/blob/master/fasthttpadaptor/adaptor.go#L49
// with additional context.Context param for use standard Go context to net/http request context
func WrapNetHTTPToFastHTTPHandler(ctx context.Context, h http.Handler) fasthttp.RequestHandler {
	return func(c *fasthttp.RequestCtx) {
		var r http.Request

		body := c.PostBody()
		r.Method = string(c.Method())
		r.Proto = "HTTP/1.1"
		r.ProtoMajor = 1
		r.ProtoMinor = 1
		r.RequestURI = string(c.RequestURI())
		r.ContentLength = int64(len(body))
		r.Host = string(c.Host())
		r.RemoteAddr = c.RemoteAddr().String()

		hdr := make(http.Header)
		c.Request.Header.VisitAll(func(k, v []byte) {
			sk := string(k)
			sv := string(v)
			switch sk {
			case "Transfer-Encoding":
				r.TransferEncoding = append(r.TransferEncoding, sv)
			default:
				hdr.Set(sk, sv)
			}
		})
		r.Header = hdr
		r.Body = &netHTTPRequestBody{body}
		rURL, err := url.ParseRequestURI(r.RequestURI)
		if err != nil {
			c.Logger().Printf("cannot parse requestURI %q: %s", r.RequestURI, err)
			c.Error("Internal Server Error", fasthttp.StatusInternalServerError)
			return
		}
		r.URL = rURL

		var w netHTTPResponseWriter
		h.ServeHTTP(&w, r.WithContext(ctx))

		c.SetStatusCode(w.StatusCode())
		haveContentType := false
		for k, vv := range w.Header() {
			if k == fasthttp.HeaderContentType {
				haveContentType = true
			}

			for _, v := range vv {
				c.Response.Header.Set(k, v)
			}
		}
		if !haveContentType {
			// From net/http.ResponseWriter.Write:
			// If the Header does not contain a Content-Type line, Write adds a Content-Type set
			// to the result of passing the initial 512 bytes of written data to DetectContentType.
			l := 512
			if len(w.body) < 512 {
				l = len(w.body)
			}
			c.Response.Header.Set(fasthttp.HeaderContentType, http.DetectContentType(w.body[:l]))
		}
		c.Write(w.body) //nolint:errcheck
	}
}

type netHTTPResponseWriter struct {
	statusCode int
	h          http.Header
	body       []byte
}

func (w *netHTTPResponseWriter) StatusCode() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *netHTTPResponseWriter) Header() http.Header {
	if w.h == nil {
		w.h = make(http.Header)
	}
	return w.h
}

func (w *netHTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *netHTTPResponseWriter) Write(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}

type netHTTPRequestBody struct {
	b []byte
}

func (r *netHTTPRequestBody) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func (r *netHTTPRequestBody) Close() error {
	r.b = r.b[:0]
	return nil
}

// FastHTTPParseContext get standard Go context from fasthttp request context
func FastHTTPParseContext(c *fasthttp.RequestCtx) context.Context {
	ctx, ok := c.UserValue(contextKey).(context.Context)
	if !ok {
		return c
	}
	return ctx
}

// FastHTTPSetContext set standard Go context to fasthttp request context
func FastHTTPSetContext(ctx context.Context, c *fasthttp.RequestCtx) {
	c.SetUserValue(contextKey, ctx)
}
