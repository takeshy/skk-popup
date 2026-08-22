// Package assetserver provides an AssetServer middleware that serves an
// external dictionary file in place of the binary-embedded one.
package assetserver

import (
	"net/http"
	"os"
)

// DictionaryMiddleware returns middleware that serves externalPath for
// /dictionary.json when the file exists, deferring everything else (and
// when it does not) to the embedded asset handler.
func DictionaryMiddleware(externalPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/dictionary.json" && externalPath != "" {
				if info, err := os.Stat(externalPath); err == nil && !info.IsDir() {
					http.ServeFile(w, r, externalPath)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
