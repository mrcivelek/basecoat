package frontend

import (
	"log"
	"net/http"
	"strings"

	"os"
	"time"

	"github.com/clintjedwards/basecoat/config"
	"github.com/clintjedwards/basecoat/utils"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

//Frontend represents an instance of the frontend application
type Frontend struct{}

//NewFrontend initializes a new UI application
func NewFrontend() *Frontend {
	return &Frontend{}
}

//historyModeHandler is a hack so that our frontend can use history mode
// We essentially answer all requests for files normally but
// any other paths we just return the normal index.html file.
// https://router.vuejs.org/guide/essentials/history-mode.html
func historyModeHandler(fileServerHandler http.Handler, indexFile []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		// serve static files as normal
		if strings.Contains(path, ".") || path == "/" {
			fileServerHandler.ServeHTTP(w, req)
			return
		}

		// return index.html for any 404s
		w.Write(indexFile)
		return
	})
}

//RegisterUIRoutes registers the endpoints needed for the frontend
// with an already established router
func (ui *Frontend) RegisterUIRoutes(router *mux.Router) {

	box := packr.NewBox("./public")
	fileServerHandler := http.FileServer(box)
	indexHTMLfile, err := box.Find("./index.html")
	if err != nil {
		log.Println(err)
	}

	router.PathPrefix("/").Handler(historyModeHandler(fileServerHandler, indexHTMLfile))
}

// InitHTTPService starts a long running http service with all proper settings; TLS enabled
func InitHTTPService(config *config.Config, server *grpc.Server) {
	wrappedGrpc := grpcweb.WrapServer(server)

	router := mux.NewRouter()

	if config.Frontend.Enable {
		frontend := NewFrontend()
		frontend.RegisterUIRoutes(router)
		utils.Log().Infow("basecoat frontend enabled", "enabled", config.Frontend)
	}

	combinedHandler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(req) {
			wrappedGrpc.ServeHTTP(resp, req)
			return
		}
		router.ServeHTTP(resp, req)
	})

	httpServer := http.Server{
		Addr:         config.Backend.HTTPURL,
		Handler:      combinedHandler,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Set default http headers
	httpServer.Handler = defaultHeaders(httpServer.Handler)

	if config.Debug {
		httpServer.Handler = handlers.LoggingHandler(os.Stdout, httpServer.Handler)
	}

	utils.Log().Infow("starting basecoat http service", "url", config.Backend.HTTPURL)
	log.Fatal(httpServer.ListenAndServeTLS(config.TLSCertPath, config.TLSKeyPath))
}

// Wrapper function setting http headers
func defaultHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		next.ServeHTTP(w, r)
	})
}
