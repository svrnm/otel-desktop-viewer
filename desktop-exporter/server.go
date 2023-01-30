package desktopexporter

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// Maximum number of traces to keep in memory
const maxNumTraces = 10000

//go:embed static/*
var assets embed.FS

type Server struct {
	server     http.Server
	traceStore *TraceStore
}

func tracesHandler(store *TraceStore) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		// Determine how many recent traces to display
		numTraces := len(store.traceMap)
		if numTraces > maxNumTraces {
			numTraces = maxNumTraces
		}

		// Get the TraceData for the requested number of traces
		traces := store.GetRecentTraces(numTraces)
		summaries := RecentSummaries{
			TraceSummaries: []TraceSummary{},
		}

		// Generate a summary for each trace
		for _, trace := range traces {
			summary := trace.GetTraceSummary()
			summaries.TraceSummaries = append(summaries.TraceSummaries, summary)
		}

		// Marshal the TraceSummaries struct and wish it well on its journey to the kingdom of frontend.
		jsonTraceSummaries, err := json.Marshal(summaries)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		} else {
			writer.WriteHeader(http.StatusOK)
			writer.Header().Set("Content-Type", "application/json")
			writer.Write(jsonTraceSummaries)
		}
	}
}

func traceIDHandler(store *TraceStore) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		traceID := mux.Vars(request)["id"]

		traceData, err := store.GetTrace(traceID)
		if err != nil {
			fmt.Println(err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		jsonTraceData, err := json.Marshal(traceData)
		if err != nil {
			fmt.Println(err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}

		writer.WriteHeader(http.StatusOK)
		writer.Header().Set("Content-Type", "application/json")
		writer.Write(jsonTraceData)
	}
}

func indexHandler(writer http.ResponseWriter, request *http.Request) {
	if os.Getenv("SERVE_FROM_FS") == "true" {
		http.ServeFile(writer, request, "./desktop-exporter/static/index.html")
	} else {
		indexBytes, err := assets.ReadFile("static/index.html")
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.Write(indexBytes)
	}
}

func NewServer(traceStore *TraceStore) *Server {
	router := mux.NewRouter()
	router.HandleFunc("/api/traces", tracesHandler(traceStore))
	router.HandleFunc("/api/traces/{id}", traceIDHandler(traceStore))
	router.HandleFunc("/traces/{id}", indexHandler)
	if os.Getenv("SERVE_FROM_FS") == "true" {
		router.PathPrefix("/").Handler(http.FileServer(http.Dir("./desktop-exporter/static/")))
	} else {
		staticContent, err := fs.Sub(assets, "static")
		if err != nil {
			log.Fatal(err)
		}
		router.PathPrefix("/").Handler(http.FileServer(http.FS(staticContent)))
	}
	return &Server{
		server: http.Server{
			Addr:    "localhost:8000",
			Handler: router,
		},
		traceStore: traceStore,
	}
}

func (s Server) Start() error {
	return s.server.ListenAndServe()
}

func (s Server) Close() error {
	return s.server.Close()
}
