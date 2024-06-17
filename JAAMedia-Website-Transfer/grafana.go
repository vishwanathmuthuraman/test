package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
	//"golang.org/x/oauth2"
	//"google.golang.org/api/idtoken"
	"clicktrack/routes"
)

// var port = ":8080"
var backend = "grafana-wba3hwcuha-uc.a.run.app"

//func main() {
//	logger := log.New(os.Stdout, "proxy: ", log.LstdFlags)
//	logger.Println(fmt.Sprintf("Proxy server is starting for: %s on port: %s", backend, port))
//
//	router := http.NewServeMux()
//	router.Handle("/", proxyHandler())
//
//	server := &http.Server{
//		Addr:         port,
//		Handler:      logging(logger)(router),
//		ErrorLog:     logger,
//		ReadTimeout:  30 * time.Second,
//		WriteTimeout: 30 * time.Second,
//		IdleTimeout:  15 * time.Second,
//	}
//
//	done := make(chan bool)
//	quit := make(chan os.Signal, 1)
//	signal.Notify(quit, os.Interrupt)
//
//	go func() {
//		<-quit
//		logger.Println("Proxy server is shutting down...")
//
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		server.SetKeepAlivesEnabled(false)
//		if err := server.Shutdown(ctx); err != nil {
//			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
//		}
//		close(done)
//	}()
//
//	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
//		logger.Fatalf("Could not listen on %s: %v\n", port, err)
//	}
//
//	<-done
//	logger.Println("Server stopped")
//}

func replace(n *html.Node) {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "class" && a.Val == "css-60onds" {
				n.Attr = append(n.Attr, html.Attribute{
					Key: "style",
					Val: "padding-top: 0 !important;",
				})
			} else if a.Key == "class" && a.Val == "css-srjygq" {
				n.Attr = append(n.Attr, html.Attribute{
					Key: "style",
					Val: "display: none !important;",
				})
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		replace(child)
	}
}

func ProxyHandler(db *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, originalR *http.Request) {
		conn, err := db.Acquire(originalR.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer conn.Release()

		if originalR.Method == http.MethodOptions {
			headers := w.Header()
			headers.Add("Access-Control-Allow-Origin", "*")
			headers.Add("Access-Control-Allow-Headers", "*")
			headers.Add("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")
			w.WriteHeader(http.StatusOK)
			return
		}

		login, err := routes.GetLogin(originalR, conn.Conn())
		if err != nil {
			http.Redirect(w, originalR, "/?redirect="+url.QueryEscape(originalR.URL.String()), http.StatusFound)

			return
		}
		originalR.Header.Set("X-WEBAUTH-USER", login.Email)
		originalR.Header.Set("X-WEBAUTH-NAME", login.Username)

		//path := fmt.Sprintf("https://%s%s", backend, r.RequestURI)
		//at, _ := idTokenTokenSource(path)

		p := httputil.NewSingleHostReverseProxy(&url.URL{
			Scheme: "https",
			Host:   backend,
		})

		p.Director = func(r *http.Request) {

		}

		p.ModifyResponse = func(res *http.Response) error {
			res.Header.Set("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")
			res.Header.Set("Access-Control-Allow-Credentials", "true")
			res.Header.Set("Access-Control-Allow-Origin", "*")
			res.Header.Set("Access-Control-Allow-Headers", "*")

			if res.Header.Get("Content-Type") == "text/html; charset=UTF-8" {
				root, err := html.Parse(res.Body)
				if err != nil {
					return errors.New(fmt.Sprintf("error parsing grafana response: %s", err.Error()))
				}

				var found func(*html.Node)
				found = func(n *html.Node) {
					if n.Type == html.ElementNode && n.Data == "head" {
						styleOverride := &html.Node{
							Type: html.ElementNode,
							Data: "style",
						}
						styleContent := &html.Node{
							Type: html.TextNode,
							Data: `
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

html, body {
    font-family: 'Inter', -apple-system, sans-serif !important;
}
.css-60onds {
	padding-top: 50px !important;
}

.css-on8iy0 > *:not(.css-f6xxc0-NavToolbar-actions) {
	display: none !important;
}

.css-1peyh2t, .css-15okvyg {
	display: none !important;
}

.css-f6xxc0-NavToolbar-actions > *:not(.css-63jktz) {
	display: none !important;
}

.css-ke2n4t-panel-container {
    border-radius: 10px;
}

.css-srjygq {
    border-radius: 10px;

`,
						}
						styleOverride.AppendChild(styleContent)
						n.AppendChild(styleOverride)
					}
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						found(c)
					}
				}
				found(root)

				// after rewriting
				//for child := root.FirstChild; child != nil; child = child.NextSibling {
				var buf bytes.Buffer
				temp := io.Writer(&buf)
				if err = html.Render(temp, root); err != nil {
					return fmt.Errorf("while rendering new html: %s", err.Error())
				}
				//}
				res.Body = io.NopCloser(bytes.NewReader([]byte(buf.String())))
			}

			return nil
		}

		originalR.URL.Scheme = "https"
		originalR.URL.Host = backend
		originalR.Header.Set("X-Forwarded-Host", originalR.Header.Get("Host"))
		originalR.Host = backend

		//if at != nil {
		//at.SetAuthHeader(r)
		//}

		p.ServeHTTP(w, originalR)
	})
}

func logging(l *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestId := r.Header.Get("X-Request-Id")
				if requestId == "" {
					requestId = fmt.Sprintf("%d", time.Now().UnixNano())
				}
				w.Header().Set("X-Request-Id", requestId)
				l.Println(requestId, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()

			next.ServeHTTP(w, r)
		})
	}
}

//func idTokenTokenSource(audience string) (*oauth2.Token, error) {
//	ts, err := idtoken.NewTokenSource(context.Background(), audience)
//	if err != nil {
//		return nil, err
//	}
//
//	t, err := ts.Token()
//	if err != nil {
//		return nil, err
//	}
//
//	return t, nil
//}
