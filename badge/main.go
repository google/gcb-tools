/*
   Copyright 2022 Google LLC

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       https://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/googlestaging/gcb-tools/badge/internal/json"
	"github.com/googlestaging/gcb-tools/badge/internal/svg"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"github.com/gorilla/mux"
)

func main() {
	ttfPath := flag.String("ttf_path",
		"third_party/googlefonts/opensans/fonts/ttf/OpenSans-Regular.ttf",
		"Path to OpenSans TTF Font file.")

	flag.Parse()
	cwd, _ := os.Getwd()
	log.Printf("starting server in '%v' ...", cwd)

	if _, err := os.Stat(*ttfPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("Font File: '%s' does not exist.", *ttfPath)
		}
		log.Fatalf("Font File: '%s' does not exist.", *ttfPath)
	}
	ctx := context.Background()
	c, err := cloudbuild.NewClient(ctx)
	if err != nil {
		log.Fatalf("Unable to create Cloud Build Client: %v", err.Error())
	}
	defer c.Close()

	r := mux.NewRouter()
	r.HandleFunc("/{projectId}/status.json", json.Handle(ctx, c))
	r.HandleFunc("/{projectId}/status.svg", svg.Handle(ctx, c, *ttfPath))

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
